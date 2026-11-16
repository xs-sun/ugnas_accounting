package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/service"
)

type SnapshotHandler struct {
	snapshotService *service.SnapshotService
}

func NewSnapshotHandler(snapshotService *service.SnapshotService) *SnapshotHandler {
	return &SnapshotHandler{snapshotService: snapshotService}
}

// ListSnapshots godoc
// @Summary 获取历史热快照备份列表
// @Description 获取本地存储的所有 SQLite 在线热快照及元数据信息（大小、时间、校验码）
// @Tags Snapshots
// @Accept json
// @Produce json
// @Success 200 {object} Response{data=[]model.Snapshot} "快照列表"
// @Router /api/v1/snapshots [get]
func (h *SnapshotHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	snaps, err := h.snapshotService.ListSnapshots(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if snaps == nil {
		snaps = []model.Snapshot{}
	}
	sendJSON(w, 0, http.StatusOK, "success", snaps)
}

// CreateSnapshot godoc
// @Summary 创建在线无锁热快照
// @Description 触发 SQLite WAL 模式无锁热快照（VACUUM INTO 原生指令），保证业务不中断并打包小票附件
// @Tags Snapshots
// @Accept json
// @Produce json
// @Success 201 {object} Response "快照创建成功"
// @Failure 500 {object} Response "快照生成失败"
// @Router /api/v1/snapshots [post]
func (h *SnapshotHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	uCtx := GetUserContext(r)
	snap, err := h.snapshotService.CreateSnapshot(r.Context(), uCtx.ID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to create snapshot: "+err.Error())
		return
	}

	sendJSON(w, 0, http.StatusCreated, "success", map[string]interface{}{
		"id":        snap.ID,
		"file_name": snap.FileName,
		"file_size": snap.FileSize,
		"sha256":    snap.SHA256,
	})
}

// DownloadSnapshot godoc
// @Summary 下载热快照文件
// @Description 下载包含数据库与小票附件的加密完整快照包
// @Tags Snapshots
// @Accept json
// @Produce application/octet-stream
// @Param id path string true "快照 ID"
// @Success 200 {file} file "快照文件流"
// @Failure 404 {object} Response "快照不存在"
// @Router /api/v1/snapshots/{id}/download [get]
func (h *SnapshotHandler) DownloadSnapshot(w http.ResponseWriter, r *http.Request, snapID string) {
	snap, err := h.snapshotService.GetSnapshot(r.Context(), snapID)
	if err != nil || snap == nil {
		sendError(w, http.StatusNotFound, "snapshot not found")
		return
	}

	if _, err := os.Stat(snap.FilePath); os.IsNotExist(err) {
		sendError(w, http.StatusNotFound, "snapshot file missing on disk")
		return
	}

	data, err := os.ReadFile(snap.FilePath)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to read snapshot file: "+err.Error())
		return
	}

	sendBytes(w, data, "application/octet-stream", map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, snap.FileName),
	}, http.StatusOK)
}

// RestoreSnapshot godoc
// @Summary 极速恢复热快照
// @Description 上传快照备份文件进行解压与数据热恢复覆盖
// @Tags Snapshots
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "快照归档文件"
// @Success 200 {object} Response "恢复成功"
// @Failure 400 {object} Response "快照文件损坏或版本不兼容"
// @Router /api/v1/snapshots/restore [post]
func (h *SnapshotHandler) RestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 100 * 1024 * 1024 // 100MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		sendError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		sendError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	archiveBytes, err := io.ReadAll(file)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to read archive: "+err.Error())
		return
	}

	if err := h.snapshotService.RestoreSnapshot(r.Context(), archiveBytes); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	sendJSON(w, 0, http.StatusOK, "restore successful", nil)
}

// DeleteSnapshot godoc
// @Summary 删除热快照
// @Description 从磁盘中永久清理历史热快照文件
// @Tags Snapshots
// @Accept json
// @Produce json
// @Param id path string true "快照 ID"
// @Success 200 {object} Response "删除成功"
// @Router /api/v1/snapshots/{id} [delete]
func (h *SnapshotHandler) DeleteSnapshot(w http.ResponseWriter, r *http.Request, snapID string) {
	if err := h.snapshotService.DeleteSnapshot(r.Context(), snapID); err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, 0, http.StatusOK, "success", nil)
}
