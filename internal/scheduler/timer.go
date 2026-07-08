// Package scheduler — таймер автоотключения WG.
//
// Когда пользователь включает WG, ставится таймер на N минут.
// Если пользователь выключает WG вручную — таймер отменяется.
// Если пользователь продлевает — старый таймер отменяется, ставится новый.
//
// Состояние хранится в ОЗУ (бот — единственный процесс).
// State file опционально для восстановления после перезапуска.
package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Action — что делать по истечении таймера.
type Action func()

// Timer управляет таймером автоотключения.
type Timer struct {
	mu      sync.Mutex
	timer   *time.Timer
	remaining time.Duration
	active  bool
	action  Action
}

// New создаёт новый scheduler.Timer.
// action будет вызвана, когда таймер сработает.
func New(action Action) *Timer {
	return &Timer{action: action}
}

// Start запускает таймер на duration. Предыдущий таймер отменяется.
func (t *Timer) Start(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.StopLocked()

	t.remaining = d
	t.active = true
	t.timer = time.AfterFunc(d, func() {
		t.mu.Lock()
		t.active = false
		t.mu.Unlock()
		if t.action != nil {
			t.action()
		}
	})
	log.Printf("[scheduler] auto-off timer started: %v", d)
}

// Stop отменяет таймер.
func (t *Timer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.StopLocked()
}

// StopLocked — внутренняя, без захвата мьютекса (для вызова из Locked-блока).
func (t *Timer) StopLocked() {
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	t.active = false
	log.Printf("[scheduler] auto-off timer stopped")
}

// IsActive возвращает true, если таймер запущен.
func (t *Timer) IsActive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active
}

// Remaining возвращает оставшееся время.
func (t *Timer) Remaining() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.active || t.timer == nil {
		return 0
	}
	return t.remaining
}

// String — человекочитаемое представление.
func (t *Timer) String() string {
	if !t.IsActive() {
		return "not running"
	}
	r := t.Remaining()
	return fmt.Sprintf("auto-off in %v", r.Round(time.Second))
}
