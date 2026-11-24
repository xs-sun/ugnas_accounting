package service

import (
	"testing"
	"time"
)

// TestHubPublishSubscribe：订阅某账本后只收到该账本的事件，且 seq 从 1 开始单调递增。
func TestHubPublishSubscribe(t *testing.T) {
	h := NewHub()

	ch, unsub := h.Subscribe("book-a")
	defer unsub()

	h.Publish("book-a", ChangeTransaction, "小明")
	h.Publish("book-b", ChangeTransaction, "小明")

	select {
	case ev := <-ch:
		if ev.Type != "ledger-changed" {
			t.Fatalf("event type = %q, want ledger-changed", ev.Type)
		}
		if ev.BookID != "book-a" {
			t.Fatalf("event book_id = %q, want book-a", ev.BookID)
		}
		if ev.Seq != 1 {
			t.Fatalf("event seq = %d, want 1", ev.Seq)
		}
		if ev.Change != ChangeTransaction {
			t.Fatalf("event change = %q, want transaction", ev.Change)
		}
		if ev.ActorName != "小明" {
			t.Fatalf("event actor_name = %q, want 小明", ev.ActorName)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for book-a event")
	}

	// book-b 的事件不应出现在 book-a 的订阅里。
	select {
	case ev := <-ch:
		t.Fatalf("unexpected event on book-a subscription: %+v", ev)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestHubSeqIsPerBook：每个账本的 seq 独立计数。
func TestHubSeqIsPerBook(t *testing.T) {
	h := NewHub()

	chA, unsubA := h.Subscribe("book-a")
	defer unsubA()
	chB, unsubB := h.Subscribe("book-b")
	defer unsubB()

	h.Publish("book-a", ChangeWallet, "小明")
	h.Publish("book-a", ChangeBudget, "小明")
	h.Publish("book-b", ChangeTransaction, "小明")

	ev := <-chA
	if ev.Seq != 1 {
		t.Fatalf("book-a first seq = %d, want 1", ev.Seq)
	}
	ev = <-chA
	if ev.Seq != 2 {
		t.Fatalf("book-a second seq = %d, want 2", ev.Seq)
	}
	ev = <-chB
	if ev.Seq != 1 {
		t.Fatalf("book-b first seq = %d, want 1", ev.Seq)
	}
}

// TestHubSeq：无订阅时 seq 也递增，之后订阅的客户端能从 connected 事件拿到当前值。
func TestHubSeq(t *testing.T) {
	h := NewHub()
	h.Publish("book-a", ChangeTransaction, "小明")
	h.Publish("book-a", ChangeTransaction, "小明")
	if got := h.Seq("book-a"); got != 2 {
		t.Fatalf("Seq = %d, want 2", got)
	}
	if got := h.Seq("book-b"); got != 0 {
		t.Fatalf("Seq(book-b) = %d, want 0", got)
	}
}

// TestHubUnsubscribe：取消订阅后不再收到事件，计数归零。
func TestHubUnsubscribe(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("book-a")
	if h.SubscriberCount("book-a") != 1 {
		t.Fatalf("SubscriberCount = %d, want 1", h.SubscriberCount("book-a"))
	}
	unsub()
	if h.SubscriberCount("book-a") != 0 {
		t.Fatalf("SubscriberCount after unsubscribe = %d, want 0", h.SubscriberCount("book-a"))
	}
	h.Publish("book-a", ChangeTransaction, "小明")
	select {
	case ev := <-ch:
		t.Fatalf("received event after unsubscribe: %+v", ev)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestHubSlowSubscriberDoesNotBlockPublish：订阅者不读时，Publish 不能阻塞写请求。
func TestHubSlowSubscriberDoesNotBlockPublish(t *testing.T) {
	h := NewHub()
	_, unsub := h.Subscribe("book-a")
	defer unsub()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			h.Publish("book-a", ChangeTransaction, "小明")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Publish blocked by a slow subscriber (buffer full)")
	}
}
