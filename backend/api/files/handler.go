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
	"github.com/serverhub/serverhub/domain"
	"github.com/serverhub/serverhub/pkg/fsclient"
	"github.com/serverhub/serverhub/pkg/resp"
	"github.com/serverhub/serverhub/pkg/runner"
	"github.com/serverhub/serverhub/repo"
)

const maxReadSize = 2 * 1024 * 1024 // 2 MB

type FileEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

func RegisterRoutes(r *gin.RouterGroup, db repo.DB, cfg *config.Config) {
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

// ctx loads the server, an FS client (sftp or local) and a Runner (for chmod/rm
// where shelling out is simpler than walking the FS). Writes the response on error.
func ctx(c *gin.Context, db repo.DB, cfg *config.Config) (*domain.Server, fsclient.Client, runner.Runner, bool) {
	sid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		resp.BadRequest(c, "服务器 ID 无效")
		return nil, nil, nil, false
	}
	s, err := repo.GetServerByID(c.Request.Context(), db, uint(sid))
	if err != nil {
		resp.NotFound(c, "服务器不存在")
		return nil, nil, nil, false
	}
	fc, err := fsclient.For(&s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "文件客户端获取失败: "+err.Error())
		return nil, nil, nil, false
	}
	rn, err := runner.For(&s, cfg)
	if err != nil {
		resp.Fail(c, http.StatusServiceUnavailable, 5003, "执行器获取失败: "+err.Error())
		return nil, nil, nil, false
	}
	return &s, fc, rn, true
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ── handlers ─────────────────────────────────────────────────────────────────

func listHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, fc, _, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		dirPath := c.DefaultQuery("path", "/")
		entries, err := fc.ReadDir(dirPath)
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

func contentGetHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, fc, _, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		filePath := c.Query("path")
		if filePath == "" {
			resp.BadRequest(c, "路径不能为空")
			return
		}
		f, err := fc.Open(filePath)
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

func contentPutHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, fc, _, ok := ctx(c, db, cfg)
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
		f, err := fc.Create(body.Path)
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

func downloadHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, fc, _, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		filePath := c.Query("path")
		if filePath == "" {
			resp.BadRequest(c, "路径不能为空")
			return
		}
		f, err := fc.Open(filePath)
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

func uploadHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, fc, _, ok := ctx(c, db, cfg)
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
		dst, err := fc.Create(dest)
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

func mkdirHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, fc, _, ok := ctx(c, db, cfg)
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
		if err := fc.MkdirAll(body.Path); err != nil {
			resp.InternalError(c, "创建目录失败: "+err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func deleteHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, rn, ok := ctx(c, db, cfg)
		if !ok {
			return
		}
		filePath := c.Query("path")
		if filePath == "" {
			resp.BadRequest(c, "路径不能为空")
			return
		}
		out, err := rn.Run("rm -rf " + shellQuote(filePath))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}

func renameHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, fc, _, ok := ctx(c, db, cfg)
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
		if err := fc.Rename(body.OldPath, body.NewPath); err != nil {
			resp.InternalError(c, "重命名失败: "+err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

func chmodHandler(db repo.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, rn, ok := ctx(c, db, cfg)
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
		out, err := rn.Run("chmod " + shellQuote(body.Mode) + " " + shellQuote(body.Path))
		if err != nil {
			resp.InternalError(c, strings.TrimSpace(out))
			return
		}
		resp.OK(c, nil)
	}
}
