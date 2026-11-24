package service

import (
	"context"
	"testing"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
	"com.xsguyu.accounting/backend/internal/repository"
)

func setupActivityRecorderTest(t *testing.T, budget int64) (*ActivityRecorder, *repository.TransactionRepo, string, string) {
	t.Helper()
	ctx := context.Background()
	db, err := repository.InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	userRepo := repository.NewUserRepo(db)
	bookRepo := repository.NewBookRepo(db)
	walletRepo := repository.NewWalletRepo(db)
	txRepo := repository.NewTransactionRepo(db)
	budgetRepo := repository.NewBudgetRepo(db)
	activityRepo := repository.NewActivityRepo(db)
	_ = userRepo

	book := &model.Book{Name: "测试账本", OwnerID: "u1", Budget: budget}
	if err := bookRepo.CreateBook(ctx, book, "小禾"); err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	wallet := &model.Wallet{BookID: book.ID, Type: "custom", Name: "微信"}
	if err := walletRepo.CreateWallet(ctx, wallet); err != nil {
		t.Fatalf("CreateWallet: %v", err)
	}
	hub := NewHub()
	recorder := NewActivityRecorder(activityRepo, budgetRepo, hub)
	return recorder, txRepo, book.ID, wallet.ID
}

func addExpense(t *testing.T, txRepo *repository.TransactionRepo, bookID, walletID string, amount int64) {
	t.Helper()
	tx := &model.Transaction{
		BookID: bookID, Type: "expense", Amount: amount,
		Date: time.Now().Format("2006-01-02"), WalletID: walletID,
		Title: "测试", IncludeBudget: true, CreatedBy: "u1",
	}
	if err := txRepo.CreateTransaction(context.Background(), tx, nil); err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}
}

// 一笔直接从 0 跳到 105%：只发一条 100% 提醒（不补发 80%），且两档都记为已提醒。
func TestCheckBudgetsJumpOverBothLevels(t *testing.T) {
	recorder, txRepo, bookID, walletID := setupActivityRecorderTest(t, 10000)
	ctx := context.Background()

	addExpense(t, txRepo, bookID, walletID, 10500)
	alerts := recorder.CheckBudgets(ctx, bookID)
	if len(alerts) != 1 {
		t.Fatalf("alerts = %d, want 1", len(alerts))
	}
	if alerts[0].Level != 100 {
		t.Fatalf("level = %d, want 100", alerts[0].Level)
	}
	// 两档都已记为已提醒：再次检查不再提醒
	if again := recorder.CheckBudgets(ctx, bookID); len(again) != 0 {
		t.Fatalf("second check alerts = %d, want 0", len(again))
	}
}

// 逐步越线：80% 与 100% 各提醒一次。
func TestCheckBudgetsGradualCrossing(t *testing.T) {
	recorder, txRepo, bookID, walletID := setupActivityRecorderTest(t, 10000)
	ctx := context.Background()

	addExpense(t, txRepo, bookID, walletID, 8000)
	if alerts := recorder.CheckBudgets(ctx, bookID); len(alerts) != 1 || alerts[0].Level != 80 {
		t.Fatalf("after 80%%: alerts = %+v, want one level-80", alerts)
	}
	addExpense(t, txRepo, bookID, walletID, 1000)
	if alerts := recorder.CheckBudgets(ctx, bookID); len(alerts) != 0 {
		t.Fatalf("after 90%%: alerts = %+v, want none", alerts)
	}
	addExpense(t, txRepo, bookID, walletID, 1000)
	if alerts := recorder.CheckBudgets(ctx, bookID); len(alerts) != 1 || alerts[0].Level != 100 {
		t.Fatalf("after 100%%: alerts = %+v, want one level-100", alerts)
	}
}

// Record 写动态失败不影响主流程：recorder 的 repo 指向已关闭的库时仍应推送 SSE。
func TestRecordDoesNotBlockOnDBError(t *testing.T) {
	ctx := context.Background()
	db, err := repository.InitDB(t.TempDir())
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	activityRepo := repository.NewActivityRepo(db)
	hub := NewHub()
	recorder := NewActivityRecorder(activityRepo, nil, hub)
	ch, unsub := hub.Subscribe("book1")
	defer unsub()
	_ = db.Close() // 写库必然失败

	recorder.Record(ctx, model.BookActivity{
		BookID: bookIDForTest, UserID: "u1", ActorName: "小禾",
		Action: model.ActivityTxCreated, Summary: "记了一笔支出「午餐」¥32.00",
	}, ChangeTransaction)

	select {
	case ev := <-ch:
		if ev.Summary == "" {
			t.Fatalf("event has no summary")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no SSE event published despite DB failure")
	}
}

const bookIDForTest = "book1"
