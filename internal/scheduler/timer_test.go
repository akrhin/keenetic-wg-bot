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

	timer.Start(50 * time.Millisecond)
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
	triggered := false
	timer := New(func() {
		triggered = true
	})

	timer.Start(1 * time.Hour) // очень долго
	timer.Stop()

	if timer.IsActive() {
		t.Error("expected inactive after stop")
	}

	// Ждём немного, чтобы убедиться, что таймер не сработал
	time.Sleep(50 * time.Millisecond)
	if triggered {
		t.Fatal("timer triggered despite stop")
	}
}

func TestTimer_Restart(t *testing.T) {
	triggered := make(chan struct{}, 2)
	timer := New(func() {
		triggered <- struct{}{}
	})

	// Запускаем с очень долгим таймером
	timer.Start(1 * time.Hour)

	// Тут же перезапускаем на короткий
	timer.Start(50 * time.Millisecond)

	// Должен сработать ровно 1 раз (второй таймер)
	select {
	case <-triggered:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("restarted timer did not trigger")
	}

	// Проверяем, что не было второго срабатывания
	select {
	case <-triggered:
		t.Fatal("timer triggered more than once")
	case <-time.After(50 * time.Millisecond):
		// ok — ровно один
	}
}
