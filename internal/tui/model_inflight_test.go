package tui

import (
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestInFlightActionsReportStatusInsteadOfSilentlyIgnoringKeys(t *testing.T) {
	for _, test := range []struct {
		name, key, want string
		view            ViewName
		configure       func(*Model)
	}{
		{name: "restore", key: "s", want: "restore already running", view: ViewActive, configure: func(m *Model) { m.restoreInFlight = true }},
		{name: "sync", key: "S", want: "sync already running", view: ViewActive, configure: func(m *Model) { m.syncInFlight = true }},
		{name: "doctor", key: "f", want: "doctor fix already running", view: ViewDoctor, configure: func(m *Model) { m.doctorFixInFlight = true }},
		{name: "rename", key: keyRepoRename, want: "rename already running", view: ViewRepo, configure: func(m *Model) { m.renameInFlight = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := New(config.Default(t.TempDir(), t.TempDir()))
			m.setView(test.view)
			test.configure(&m)
			updated, _ := m.Update(keyRunes(test.key))
			got := mustModel(t, updated)
			if got.status != test.want {
				t.Fatalf("status = %q, want %q", got.status, test.want)
			}
		})
	}
}

func TestLeavingInstallClearsInstallInputMode(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.InputMode = installInputQuery
	m.setView(ViewActive)
	if m.install.InputMode != installInputNone {
		t.Fatalf("InputMode = %v, want installInputNone", m.install.InputMode)
	}
}
