package scheduler

import (
	"testing"
	"time"
)

func TestTimer_StartStop(t *testing.T) {
	triggered := make(chan struct{}, 1)
	timer := New(func() {
		triggered <- struct{}{}
	})

	if timer.IsActive() {
		t.Error("expected inactive after creation")
	}

	timer.Start(100 * time.Millisecond)
	if !timer.IsActive() {
		t.Error("expected active after start")
	}

	select {
	case <-triggered:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timer did not trigger")
	}

	if timer.IsActive() {
		t.Error("expected inactive after trigger")
	}
}

func TestTimer_Cancel(t *testing.T) {
	triggered := make(chan struct{}, 1)
	timer := New(func() {
		close(triggered)
	})

	timer.Start(1 * time.Hour) // очень долго
	timer.Stop()

	if timer.IsActive() {
		t.Error("expected inactive after stop")
	}

	// Проверяем, что не сработал
	select {
	case <-triggered:
		t.Fatal("timer triggered despite stop")
	case <-time.After(50 * time.Millisecond):
		// ok
	}
}

func TestTimer_Restart(t *testing.T) {
	count := 0
	timer := New(func() {
		count++
	})

	timer.Start(1 * time.Hour)
	timer.Start(50 * time.Millisecond) // перезапускаем раньше

	time.Sleep(100 * time.Millisecond)

	if count != 1 {
		t.Errorf("expected 1 trigger, got %d", count)
	}
}
