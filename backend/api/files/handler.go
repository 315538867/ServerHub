package files

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serverhub/serverhub/config"
	"github.com/serverhub/serverhub/model"
	"github.com/serverhub/serverhub/pkg/crypto"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/sftppool"
	"github.com/serverhub/serverhub/pkg/sshpool"
	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

const maxReadSize = 2 * 1024 * 1024 // 2 MB

type FileEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

func RegisterRoutes(r *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	r.GET("/:id/files/list", listHandler(db, cfg))
	r.GET("/:id/files/content", contentGetHandler(db, cfg))
	r.PUT("/:id/files/content", contentPutHandler(db, cfg))
	r.GET("/:id/files/download", downloadHandler(db, cfg))
	r.POST("/:id/files/upload", uploadHandler(db, cfg))
	r.POST("/:id/files/mkdir", mkdirHandler(db, cfg))
	r.DELETE("/:id/files/delete", deleteHandler(db, cfg))
	r.POST("/:id/files/rename", renameHandler(db, cfg))
	r.POST("/:id/files/chmod", chmodHandler(db, cfg))
}

// ctx resolves server ID, SSH client, and SFTP client from the request.
// Returns false and writes response if anything fails.
func ctx(c *gin.Context, db *gorm.DB, cfg *config.Config) (uint, *gossh.Client, *sftp.Client, bool) {
	sid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return 0, nil, nil, false
	}
	var s model.Server
	if err := db.First(&s, sid).Error; err != nil {
		resp.NotFound(c, "服务器不存在")
		return 0, nil, nil, false
	}
	var cred string
	switch s.AuthType {
	case "key":
		if s.PrivateKey != "" {
			cred, err = crypto.Decrypt(s.PrivateKey, cfg.Security.AESKey)
		}
	default:
		if s.Password != "" {
			cred, err = crypto.Decrypt(s.Password, cfg.Security.AESKey)
		}
	}
	if err != nil {
		resp.InternalError(c, "解密失败")
		return 0, nil, nil, false
	}
	sshClient, err := sshpool.Connect(s.ID, s.Host, s.Port, s.Username, s.AuthType, cred)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "SSH 连接失败: "+err.Error())
		return 0, nil, nil, false
	}
	sc, err := sftppool.Get(s.ID, sshClient)
	if err != nil {
		resp.InternalError(c, "SFTP 连接失败: "+err.Error())
		return 0, nil, nil, false
	}
	return s.ID, sshClient, sc, true
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ── handlers ─────────────────────────────────────────────────────────────────

func listHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, sc, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		dirPath := c.DefaultQuery("path", "/")
		entries, err := sc.ReadDir(dirPath)
		if err != nil {
			resp.InternalError(c, "读取目录失败: "+err.Error())
			return
		}
		items := make([]FileEntry, 0, len(entries))
		for _, e := range entries {
			items = append(items, FileEntry{
				Name:    e.Name(),
				IsDir:   e.IsDir(),
				Size:    e.Size(),
				Mode:    e.Mode().String(),
				ModTime: e.ModTime().Format(time.DateTime),
			})
		}
		resp.OK(c, gin.H{"path": dirPath, "entries": items})
	}
}

func contentGetHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, sc, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		filePath := c.Query("path")
		if filePath == "" {
			resp.BadRequest(c, "路径不能为空")
			return
		}
		f, err := sc.Open(filePath)
		if err != nil {
			resp.InternalError(c, "打开文件失败: "+err.Error())
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		if stat != nil && stat.Size() > maxReadSize {
			resp.Fail(c, 400, 4001, fmt.Sprintf("文件过大（%d 字节）", stat.Size()))
			return
		}
		content, err := io.ReadAll(io.LimitReader(f, maxReadSize))
		if err != nil {
			resp.InternalError(c, "读取文件失败: "+err.Error())
			return
		}
		resp.OK(c, gin.H{"path": filePath, "content": string(content)})
	}
}

func contentPutHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, sc, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Path    string `json:"path" binding:"required"`
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		f, err := sc.Create(body.Path)
		if err != nil {
			resp.InternalError(c, "创建文件失败: "+err.Error())
			return
		}
		defer f.Close()
		if _, err := f.Write([]byte(body.Content)); err != nil {
			resp.InternalError(c, "写入文件失败: "+err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func downloadHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, sc, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		filePath := c.Query("path")
		if filePath == "" {
			resp.BadRequest(c, "路径不能为空")
			return
		}
		f, err := sc.Open(filePath)
		if err != nil {
			resp.InternalError(c, "打开文件失败: "+err.Error())
			return
		}
		defer f.Close()
		name := path.Base(filePath)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(name, `"`, ``)))
		c.Header("Content-Type", "application/octet-stream")
		io.Copy(c.Writer, f) //nolint:errcheck
	}
}

func uploadHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, sc, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		dir := c.DefaultPostForm("path", "/")
		fh, err := c.FormFile("file")
		if err != nil {
			resp.BadRequest(c, "文件不能为空")
			return
		}
		src, err := fh.Open()
		if err != nil {
			resp.InternalError(c, "打开上传文件失败: "+err.Error())
			return
		}
		defer src.Close()
		dest := path.Join(dir, path.Base(fh.Filename))
		dst, err := sc.Create(dest)
		if err != nil {
			resp.InternalError(c, "创建目标文件失败: "+err.Error())
			return
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			resp.InternalError(c, "复制文件失败: "+err.Error())
			return
		}
		resp.OK(c, gin.H{"path": dest})
	}
}

func mkdirHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, sc, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Path string `json:"path" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "路径不能为空")
			return
		}
		if err := sc.MkdirAll(body.Path); err != nil {
			resp.InternalError(c, "创建目录失败: "+err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func deleteHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, sshClient, _, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		filePath := c.Query("path")
		if filePath == "" {
			resp.BadRequest(c, "路径不能为空")
			return
		}
		out, err := sshpool.Run(sshClient, "rm -rf "+shellQuote(filePath))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}

func renameHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, sc, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			OldPath string `json:"old_path" binding:"required"`
			NewPath string `json:"new_path" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		if err := sc.Rename(body.OldPath, body.NewPath); err != nil {
			resp.InternalError(c, "重命名失败: "+err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func chmodHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, sshClient, _, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		var body struct {
			Path string `json:"path" binding:"required"`
			Mode string `json:"mode" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			resp.BadRequest(c, "请求体格式错误")
			return
		}
		out, err := sshpool.Run(sshClient, "chmod "+shellQuote(body.Mode)+" "+shellQuote(body.Path))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}
