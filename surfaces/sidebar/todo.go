package sidebar

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sonroyaalmerol/snry-shell/internal/bus"
	"github.com/sonroyaalmerol/snry-shell/internal/gtkutil"
	"github.com/sonroyaalmerol/snry-shell/internal/servicerefs"
	"github.com/sonroyaalmerol/snry-shell/internal/state"
)

// newTodoWidget creates a todo list widget.
func newTodoWidget(b *bus.Bus, refs *servicerefs.ServiceRefs) gtk.Widgetter {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("todo-widget")

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	headerLabel := gtk.NewLabel("Tasks")
	headerLabel.AddCSSClass("notif-group-header")
	headerLabel.SetHExpand(true)
	header.Append(headerLabel)

	clearBtn := gtkutil.MaterialButtonWithClass("delete_sweep", "todo-clear-btn")
	clearBtn.ConnectClicked(func() {
		if refs.Todo != nil {
			refs.Todo.Clear()
		}
	})
	header.Append(clearBtn)
	box.Append(header)

	// Input row.
	inputRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	inputRow.AddCSSClass("todo-input-row")

	entry := gtk.NewEntry()
	entry.AddCSSClass("todo-entry")
	entry.SetPlaceholderText("Add task...")
	entry.SetHExpand(true)

	addBtn := gtkutil.MaterialButtonWithClass("add", "todo-add-btn")

	activate := func() {
		text := entry.Text()
		if text == "" {
			return
		}
		if refs.Todo != nil {
			refs.Todo.Add(text)
		}
		entry.SetText("")
	}

	addBtn.ConnectClicked(activate)
	entry.ConnectActivate(activate)

	inputRow.Append(entry)
	inputRow.Append(addBtn)
	box.Append(inputRow)

	// Task list.
	listBox := gtk.NewListBox()
	listBox.AddCSSClass("todo-list")
	listBox.SetSelectionMode(gtk.SelectionNone)

	b.Subscribe(bus.TopicTodo, func(e bus.Event) {
		items, ok := e.Data.([]state.TodoItem)
		if !ok {
			return
		}
		glib.IdleAdd(func() {
			gtkutil.ClearChildren(&listBox.Widget)

			for _, item := range items {
				row := newTodoItemRow(refs, item)
				listBox.Append(row)
			}
		})
	})

	box.Append(listBox)
	return box
}

func newTodoItemRow(refs *servicerefs.ServiceRefs, item state.TodoItem) gtk.Widgetter {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	row.AddCSSClass("todo-item")

	check := gtk.NewCheckButton()
	check.AddCSSClass("todo-check")
	check.SetActive(item.Done)

	textLabel := gtk.NewLabel(item.Text)
	textLabel.AddCSSClass("todo-text")
	textLabel.SetHExpand(true)
	textLabel.SetHAlign(gtk.AlignStart)
	if item.Done {
		textLabel.AddCSSClass("todo-done")
	}

	id := item.ID
	check.ConnectToggled(func() {
		if refs.Todo != nil {
			refs.Todo.Toggle(id)
		}
	})

	delBtn := gtkutil.MaterialButtonWithClass("close", "todo-del-btn")
	delBtn.ConnectClicked(func() {
		if refs.Todo != nil {
			refs.Todo.Remove(id)
		}
	})

	row.Append(check)
	row.Append(textLabel)
	row.Append(delBtn)

	return row
}
