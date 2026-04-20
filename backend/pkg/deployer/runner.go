package deployer

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/sshpool"
	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type Result struct {
	Output   string
	Success  bool
	Duration int
}

// Run executes a deployment via SSH.
// onLine is called for each stdout line (pass nil for background runs).
func Run(db *gorm.DB, cfg *config.Config, app model.Deploy, onLine func(string)) Result {
	var s model.Server
	if err := db.First(&s, app.ServerID).Error; err != nil {
		return Result{Output: "server not found", Success: false}
	}

	cred, err := decryptCred(s, cfg.Security.AESKey)
	if err != nil {
		return Result{Output: "decrypt failed", Success: false}
	}

	client, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
	if err != nil {
		return Result{Output: "ssh: " + err.Error(), Success: false}
	}

	now := time.Now()
	db.Model(&app).Updates(map[string]any{"last_run_at": now, "last_status": "running"})

	session, err := client.NewSession()
	if err != nil {
		db.Model(&app).Update("last_status", "failed")
		return Result{Output: "session: " + err.Error(), Success: false}
	}
	defer session.Close()

	cmd := BuildCmd(app, cfg.Security.AESKey)
	if onLine != nil {
		onLine("$ " + cmd)
	}

	stdout, _ := session.StdoutPipe()
	if err := session.Start(cmd); err != nil {
		db.Model(&app).Update("last_status", "failed")
		return Result{Output: "start: " + err.Error(), Success: false}
	}

	var buf strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line + "\n")
		if onLine != nil {
			onLine(line)
		}
	}

	success := session.Wait() == nil
	duration := int(time.Since(now).Seconds())
	output := buf.String()

	db.Create(&model.DeployLog{
		DeployID: app.ID,
		Output:   output,
		Status:   statusStr(success),
		Duration: duration,
	})

	if success {
		oldVersion := app.ActualVersion
		updates := map[string]any{
			"last_status": "success",
			"sync_status": "synced",
		}
		if app.DesiredVersion != "" {
			updates["previous_version"] = oldVersion
			updates["actual_version"] = app.DesiredVersion
		}
		db.Model(&app).Updates(updates)

		// Delete old image after successful version update
		if app.ImageName != "" && oldVersion != "" && oldVersion != app.DesiredVersion {
			go deleteOldImage(client, app.ImageName, oldVersion)
		}
	} else {
		db.Model(&app).Updates(map[string]any{
			"last_status": "failed",
			"sync_status": "error",
		})
	}

	return Result{Output: output, Success: success, Duration: duration}
}

// BuildCmd constructs the SSH command for the given app type.
func BuildCmd(app model.Deploy, aesKey string) string {
	envPrefix := buildEnvPrefix(app.EnvVars, aesKey)

	var parts []string
	if app.WorkDir != "" {
		parts = append(parts, fmt.Sprintf("mkdir -p %s", shellQuote(app.WorkDir)))
		parts = append(parts, fmt.Sprintf("cd %s", shellQuote(app.WorkDir)))
	}

	// Write config files (all deploy types)
	hasStartupSh := false
	if app.ConfigFiles != "" {
		var files []struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(app.ConfigFiles), &files); err == nil {
			for _, f := range files {
				encoded := base64.StdEncoding.EncodeToString([]byte(f.Content))
				parts = append(parts, fmt.Sprintf("echo %s | base64 -d > %s", shellQuote(encoded), shellQuote(f.Name)))
				if f.Name == "startup.sh" {
					hasStartupSh = true
				}
			}
			if hasStartupSh {
				parts = append(parts, "chmod +x startup.sh")
			}
		}
	}

	switch app.Type {
	case "docker-compose":
		if hasStartupSh {
			parts = append(parts, "bash startup.sh 2>&1")
		} else {
			cf := app.ComposeFile
			if cf == "" {
				cf = "docker-compose.yml"
			}
			parts = append(parts,
				fmt.Sprintf("docker compose -f %s pull --quiet 2>&1 || true", shellQuote(cf)),
				fmt.Sprintf("docker compose -f %s up -d --build 2>&1", shellQuote(cf)),
			)
		}
	default: // docker, native
		if hasStartupSh {
			parts = append(parts, "bash startup.sh 2>&1")
		} else if app.StartCmd != "" {
			parts = append(parts, app.StartCmd+" 2>&1")
		}
	}

	return "bash -c " + shellQuote(envPrefix+"set -e; "+strings.Join(parts, " && "))
}

func deleteOldImage(client *gossh.Client, imageName, version string) {
	sshpool.Run(client, fmt.Sprintf("docker rmi %s 2>/dev/null || true",
		shellQuote(imageName+":"+version)))
}

func buildEnvPrefix(envVarsEncrypted, aesKey string) string {
	if envVarsEncrypted == "" {
		return ""
	}
	decrypted, err := crypto.Decrypt(envVarsEncrypted, aesKey)
	if err != nil {
		return ""
	}
	var vars []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(decrypted), &vars); err != nil {
		return ""
	}
	var parts []string
	for _, v := range vars {
		if v.Key != "" {
			parts = append(parts, fmt.Sprintf("export %s=%s", shellQuote(v.Key), shellQuote(v.Value)))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ") + "; "
}

func statusStr(ok bool) string {
	if ok {
		return "success"
	}
	return "failed"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func decryptCred(s model.Server, aesKey string) (string, error) {
	switch s.AuthType {
	case "key":
		if s.PrivateKey == "" {
			return "", nil
		}
		return crypto.Decrypt(s.PrivateKey, aesKey)
	default:
		if s.Password == "" {
			return "", nil
		}
		return crypto.Decrypt(s.Password, aesKey)
	}
}
