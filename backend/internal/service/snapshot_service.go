package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

type SnapshotService struct {
	db           *repository.DB
	snapshotRepo *repository.SnapshotRepo
	dataDir      string
}

func NewSnapshotService(db *repository.DB, snapshotRepo *repository.SnapshotRepo, dataDir string) *SnapshotService {
	return &SnapshotService{
		db:           db,
		snapshotRepo: snapshotRepo,
		dataDir:      dataDir,
	}
}

func (s *SnapshotService) getSnapshotsDir() string {
	path := filepath.Join(s.dataDir, "snapshots")
	_ = os.MkdirAll(path, 0755)
	return path
}

func (s *SnapshotService) getAttachmentsDir() string {
	path := filepath.Join(s.dataDir, "attachments")
	_ = os.MkdirAll(path, 0755)
	return path
}

func (s *SnapshotService) CreateSnapshot(ctx context.Context, userID string) (*model.Snapshot, error) {
	now := time.Now()
	snapID := fmt.Sprintf("snap_%d_%s", now.Unix(), fmt.Sprintf("%04x", now.Nanosecond()%65536))
	snapsDir := s.getSnapshotsDir()

	rawDBBackup := filepath.Join(snapsDir, snapID+".sqlite")
	defer func() {
		if _, err := os.Stat(rawDBBackup); err == nil {
			_ = os.Remove(rawDBBackup)
		}
	}()

	// Execute native VACUUM INTO
	vacuumQuery := fmt.Sprintf("VACUUM INTO '%s';", rawDBBackup)
	if _, err := s.db.ExecContext(ctx, vacuumQuery); err != nil {
		return nil, fmt.Errorf("failed to vacuum database: %w", err)
	}

	ugledgerPath := filepath.Join(snapsDir, snapID+".ugledger")
	outFile, err := os.Create(ugledgerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add accounting.db
	dbInfo, err := os.Stat(rawDBBackup)
	if err != nil {
		return nil, fmt.Errorf("failed to stat backup db: %w", err)
	}

	dbHdr, err := tar.FileInfoHeader(dbInfo, "")
	if err != nil {
		return nil, err
	}
	dbHdr.Name = "accounting.db"
	if err := tw.WriteHeader(dbHdr); err != nil {
		return nil, err
	}

	dbF, err := os.Open(rawDBBackup)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(tw, dbF); err != nil {
		dbF.Close()
		return nil, err
	}
	dbF.Close()

	// Add attachments if directory exists
	attDir := s.getAttachmentsDir()
	if _, err := os.Stat(attDir); err == nil {
		_ = filepath.Walk(attDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || path == attDir {
				return nil
			}
			relPath, err := filepath.Rel(attDir, path)
			if err != nil {
				return nil
			}
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return nil
			}
			hdr.Name = filepath.Join("attachments", relPath)
			if err := tw.WriteHeader(hdr); err != nil {
				return nil
			}
			if !info.IsDir() {
				f, err := os.Open(path)
				if err != nil {
					return nil
				}
				defer f.Close()
				_, _ = io.Copy(tw, f)
			}
			return nil
		})
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	if err := outFile.Close(); err != nil {
		return nil, err
	}

	// Calculate SHA256 and size
	h := sha256.New()
	archiveFile, err := os.Open(ugledgerPath)
	if err != nil {
		return nil, err
	}
	size, err := io.Copy(h, archiveFile)
	archiveFile.Close()
	if err != nil {
		return nil, err
	}
	shaStr := hex.EncodeToString(h.Sum(nil))

	snap := &model.Snapshot{
		ID:        snapID,
		FileName:  snapID + ".ugledger",
		FilePath:  ugledgerPath,
		FileSize:  size,
		SHA256:    shaStr,
		CreatedBy: userID,
	}

	if err := s.snapshotRepo.CreateSnapshot(ctx, snap); err != nil {
		return nil, err
	}

	return snap, nil
}

func (s *SnapshotService) ListSnapshots(ctx context.Context) ([]model.Snapshot, error) {
	return s.snapshotRepo.ListSnapshots(ctx)
}

func (s *SnapshotService) GetSnapshot(ctx context.Context, id string) (*model.Snapshot, error) {
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return nil, errors.New("invalid snapshot id")
	}
	return s.snapshotRepo.GetSnapshot(ctx, id)
}

func (s *SnapshotService) DeleteSnapshot(ctx context.Context, id string) error {
	snap, err := s.snapshotRepo.GetSnapshot(ctx, id)
	if err == nil && snap != nil {
		if snap.FilePath != "" {
			_ = os.Remove(snap.FilePath)
		}
	}
	return s.snapshotRepo.DeleteSnapshot(ctx, id)
}

func (s *SnapshotService) RestoreSnapshot(ctx context.Context, archiveBytes []byte) error {
	tmpDir, err := os.MkdirTemp("", "ugnas_restore_*")
	if err != nil {
		return fmt.Errorf("failed to create temp restore directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpArchive := filepath.Join(tmpDir, "restore.ugledger")
	if err := os.WriteFile(tmpArchive, archiveBytes, 0600); err != nil {
		return fmt.Errorf("failed to write archive: %w", err)
	}

	archiveFile, err := os.Open(tmpArchive)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer archiveFile.Close()

	gr, err := gzip.NewReader(archiveFile)
	if err != nil {
		return fmt.Errorf("corrupted archive: not gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("corrupted archive: %w", err)
		}

		// Mitigate Zip Slip
		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue
		}
		target := filepath.Join(tmpDir, cleanName)

		if hdr.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}

		_ = os.MkdirAll(filepath.Dir(target), 0755)
		f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return fmt.Errorf("failed to write extracted file: %w", err)
		}
		f.Close()
	}

	restoredDB := filepath.Join(tmpDir, "accounting.db")
	if _, err := os.Stat(restoredDB); os.IsNotExist(err) {
		return errors.New("invalid archive: accounting.db missing")
	}

	// Verify PRAGMA integrity_check
	checkDB, err := sql.Open("sqlite", restoredDB)
	if err != nil {
		return fmt.Errorf("failed to open restored db: %w", err)
	}
	var integrityRes string
	err = checkDB.QueryRow("PRAGMA integrity_check;").Scan(&integrityRes)
	checkDB.Close()
	if err != nil || integrityRes != "ok" {
		return errors.New("sqlite integrity check failed")
	}

	// Safely copy restored DB to live DB path
	liveDBPath := filepath.Join(s.dataDir, "accounting.db")
	_ = copyFile(restoredDB, liveDBPath)

	// Restore attachments if present
	restoredAtt := filepath.Join(tmpDir, "attachments")
	if fi, err := os.Stat(restoredAtt); err == nil && fi.IsDir() {
		liveAtt := filepath.Join(s.dataDir, "attachments")
		_ = os.RemoveAll(liveAtt)
		_ = copyDir(restoredAtt, liveAtt)
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}
		return copyFile(path, destPath)
	})
}
