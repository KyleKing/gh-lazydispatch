package modal

import (
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unsafe"

	"charm.land/bubbles/v2/key"
	"github.com/kyleking/aragonite/keyusage"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
)

// recorder is implemented by modals that read free text or take dynamic keys,
// and so decide for themselves which presses are commands.
type recorder interface {
	Records(k string) bool
}

// KeyContext names the modal a key lands in, as the usage log spells it.
func KeyContext(c Context) string {
	name := strings.TrimSuffix(strings.TrimPrefix(reflect.TypeOf(c).String(), "*modal."), "Modal")

	var b strings.Builder

	for i, r := range name {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('-')
		}

		b.WriteRune(unicode.ToLower(r))
	}

	return b.String()
}

// Records reports whether key pressed in c is a command to log, never text.
func Records(c Context, k string) bool {
	if r, ok := c.(recorder); ok {
		return r.Records(k)
	}

	return true
}

// Keymap is every modal's bindings by KeyContext, read from the key tables the
// modals dispatch on. Modals that only read text have none.
func Keymap() keyusage.Keymap {
	km := keyusage.Keymap{}

	for _, c := range samples() {
		if keys, ok := keyTable(c); ok {
			km[KeyContext(c)] = Bindings(keys)
		}
	}

	return km
}

// keyTable is the unexported keys field every modal dispatches on.
func keyTable(c Context) (any, bool) {
	f := reflect.ValueOf(c).Elem().FieldByName("keys")
	if !f.IsValid() {
		return nil, false
	}

	//nolint:gosec // reads a field of a modal this package owns
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().Interface(), true
}

// Bindings reads the key.Binding fields of a key table, naming each by its help
// text, or by its field when it has none.
func Bindings(table any) []keyusage.Binding {
	v := reflect.ValueOf(table)

	var out []keyusage.Binding

	for i := range v.NumField() {
		b, ok := v.Field(i).Interface().(key.Binding)
		if !ok {
			continue
		}

		what := b.Help().Desc
		if what == "" {
			what = strings.ToLower(splitWords(v.Type().Field(i).Name))
		}

		out = append(out, keyusage.Binding{Keys: b.Keys(), What: what})
	}

	return out
}

func splitWords(name string) string {
	var b strings.Builder

	for i, r := range name {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte(' ')
		}

		b.WriteRune(r)
	}

	return b.String()
}

func bound(table any, k string) bool {
	return slices.ContainsFunc(Bindings(table), func(b keyusage.Binding) bool { return slices.Contains(b.Keys, k) })
}

// samples are one of each modal that has a key table, built from empty inputs.
func samples() []Context {
	return []Context{
		NewActionMenuModal("", "", nil),
		NewChainConfirmModal("", &config.Chain{}, nil, "", false),
		NewChainRerunModal(&frecency.HistoryEntry{}),
		NewChainSelectModal(&config.WfdConfig{}),
		NewChainStatusModal(chain.ChainState{}),
		NewChainVariableModal("", &config.Chain{}),
		NewConfirmModal("", "", false, false),
		NewErrorModal("", ""),
		NewHelpModal(),
		NewLiveViewModal(nil),
		NewLogsViewerModal(&logs.RunLogs{}, 0, 0),
		NewRemapModal(nil, nil),
		NewResetModal(nil),
		NewRunActionModal("", 0, ""),
		NewRunConfirmModal(runner.RunConfig{}),
		NewSelectModal("", "", nil, "", ""),
		NewSimpleBranchModal("", nil, "", ""),
		NewValidationErrorModal(nil),
	}
}

// Records is false for free text, and for any key the modal does not bind.
func (m *SimpleBranchModal) Records(k string) bool { return !m.filtering && bound(m.keys, k) }

// Records is false while a value is being typed.
func (m *ChainVariableModal) Records(k string) bool { return !m.editing && bound(m.keys, k) }

// Records is false while a search is being typed.
func (m *LogsViewerModal) Records(k string) bool { return !m.searchMode && bound(m.keys, k) }

// Records is false: every key but enter and esc is text.
func (*FilterModal) Records(string) bool { return false }

// Records is false: every key but enter and esc is text.
func (*InputModal) Records(string) bool { return false }
