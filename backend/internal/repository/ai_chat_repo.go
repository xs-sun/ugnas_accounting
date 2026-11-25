package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"com.xsguyu.accounting/backend/internal/model"
)

// AiChatRepo 存取服务端的 AI 聊天记录。记录按用户 + 账本隔离，
// 各人只能读写自己的对话；图片文件本身存在 <dataDir>/ai-images/，
// 这里只存文件名。
type AiChatRepo struct {
	db *DB
}

func NewAiChatRepo(db *DB) *AiChatRepo {
	return &AiChatRepo{db: db}
}

// MaxAiChatMessagesPerBook 每个用户在每个账本里最多保留的聊天条数，
// 超出时删最旧的（连同图片文件一起删，见 PruneMessages）。
const MaxAiChatMessagesPerBook = 200

func scanAiChatMessage(row interface {
	Scan(dest ...any) error
}) (*model.AiChatMessage, error) {
	var m model.AiChatMessage
	var isVoice, saved int
	var draftJSON string
	if err := row.Scan(
		&m.ID, &m.BookID, &m.UserID, &m.Role, &m.Text,
		&isVoice, &m.ImagePath, &m.ImageLabel, &m.OcrText,
		&draftJSON, &saved, &m.SavedTxID, &m.CreatedAt,
	); err != nil {
		return nil, err
	}
	m.IsVoice = isVoice != 0
	m.Saved = saved != 0
	if draftJSON != "" {
		m.Draft = json.RawMessage(draftJSON)
	}
	return &m, nil
}

const aiChatMessageColumns = `id, book_id, user_id, role, text, is_voice, image_path, image_label, ocr_text, draft_json, saved, saved_tx_id, created_at`

func (r *AiChatRepo) CreateMessage(ctx context.Context, m *model.AiChatMessage) error {
	draftJSON := ""
	if len(m.Draft) > 0 {
		draftJSON = string(m.Draft)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ai_chat_messages (`+aiChatMessageColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		m.ID, m.BookID, m.UserID, m.Role, m.Text,
		boolToInt(m.IsVoice), m.ImagePath, m.ImageLabel, m.OcrText,
		draftJSON, boolToInt(m.Saved), m.SavedTxID,
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ListMessages 按插入顺序返回该用户在该账本的聊天记录，最多 limit 条。
func (r *AiChatRepo) ListMessages(ctx context.Context, bookID, userID string, limit int) ([]*model.AiChatMessage, error) {
	if limit <= 0 || limit > MaxAiChatMessagesPerBook {
		limit = MaxAiChatMessagesPerBook
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+aiChatMessageColumns+` FROM ai_chat_messages WHERE book_id = ? AND user_id = ? ORDER BY rowid ASC LIMIT ?`,
		bookID, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.AiChatMessage
	for rows.Next() {
		m, err := scanAiChatMessage(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *AiChatRepo) GetMessage(ctx context.Context, id string) (*model.AiChatMessage, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+aiChatMessageColumns+` FROM ai_chat_messages WHERE id = ?`, id)
	m, err := scanAiChatMessage(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return m, nil
}

// UpdateMessage 更新一条记录的可变字段（OCR 文本、草稿、入账状态、图片）。
// 调用方须先用 GetMessage 核对该记录属于当前用户与账本。
func (r *AiChatRepo) UpdateMessage(ctx context.Context, m *model.AiChatMessage) error {
	draftJSON := ""
	if len(m.Draft) > 0 {
		draftJSON = string(m.Draft)
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE ai_chat_messages SET text = ?, is_voice = ?, image_path = ?, image_label = ?, ocr_text = ?, draft_json = ?, saved = ?, saved_tx_id = ? WHERE id = ?`,
		m.Text, boolToInt(m.IsVoice), m.ImagePath, m.ImageLabel, m.OcrText,
		draftJSON, boolToInt(m.Saved), m.SavedTxID, m.ID,
	)
	return err
}

// DeleteMessagesForUser 删掉该用户在该账本的全部聊天记录，返回被删记录
// 引用过的图片文件名（调用方负责删文件）。
func (r *AiChatRepo) DeleteMessagesForUser(ctx context.Context, bookID, userID string) (deleted int, imagePaths []string, err error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT image_path FROM ai_chat_messages WHERE book_id = ? AND user_id = ? AND image_path <> ''`,
		bookID, userID,
	)
	if err != nil {
		return 0, nil, err
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return 0, nil, err
		}
		imagePaths = append(imagePaths, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}

	res, err := r.db.ExecContext(ctx,
		`DELETE FROM ai_chat_messages WHERE book_id = ? AND user_id = ?`, bookID, userID)
	if err != nil {
		return 0, nil, err
	}
	n, _ := res.RowsAffected()
	return int(n), imagePaths, nil
}

// PruneMessages 删掉超出 keep 条的最旧记录，返回被删记录的图片文件名。
func (r *AiChatRepo) PruneMessages(ctx context.Context, bookID, userID string, keep int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, image_path FROM ai_chat_messages WHERE book_id = ? AND user_id = ? AND id NOT IN (
			SELECT id FROM ai_chat_messages WHERE book_id = ? AND user_id = ? ORDER BY rowid DESC LIMIT ?
		)`,
		bookID, userID, bookID, userID, keep,
	)
	if err != nil {
		return nil, err
	}
	var ids []string
	var imagePaths []string
	for rows.Next() {
		var id, p string
		if err := rows.Scan(&id, &p); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
		if p != "" {
			imagePaths = append(imagePaths, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	for _, id := range ids {
		if _, err := r.db.ExecContext(ctx, `DELETE FROM ai_chat_messages WHERE id = ?`, id); err != nil {
			return nil, err
		}
	}
	return imagePaths, nil
}
