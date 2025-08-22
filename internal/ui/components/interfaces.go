package components

import (
	"github.com/devnullvoid/peevetui/pkg/api"
	"github.com/rivo/tview"
)

type NodeListComponent interface {
	tview.Primitive
	SetApp(*App)
	SetNodes([]*api.Node)
	GetSelectedNode() *api.Node
	GetNodes() []*api.Node
	SetNodeSelectedFunc(func(*api.Node))
	SetNodeChangedFunc(func(*api.Node))
	SetCurrentItem(int) *tview.List
	GetCurrentItem() int
}

type VMListComponent interface {
	tview.Primitive
	SetApp(*App)
	SetVMs([]*api.VM)
	GetSelectedVM() *api.VM
	GetVMs() []*api.VM
	SetVMSelectedFunc(func(*api.VM))
	SetVMChangedFunc(func(*api.VM))
	SetCurrentItem(int) *tview.List
	GetCurrentItem() int
}

type NodeDetailsComponent interface {
	tview.Primitive
	SetApp(*App)
	Update(*api.Node, []*api.Node)
	Clear() *tview.Table
}

type VMDetailsComponent interface {
	tview.Primitive
	SetApp(*App)
	Update(*api.VM)
	Clear() *tview.Table
}

type TasksListComponent interface {
	tview.Primitive
	SetApp(*App)
	SetTasks([]*api.ClusterTask)
	SetFilteredTasks([]*api.ClusterTask)
	GetSelectedTask() *api.ClusterTask
	Select(row, column int) *tview.Table
}

type ClusterStatusComponent interface {
	tview.Primitive
	Update(*api.Cluster)
}

type HeaderComponent interface {
	tview.Primitive
	SetApp(*tview.Application)
	ShowLoading(string)
	StopLoading()
	IsLoading() bool
	ShowSuccess(string)
	ShowError(string)
	ShowWarning(string)
	SetTitle(string)
	ShowActiveProfile(string)
	GetCurrentProfile() string
}

type FooterComponent interface {
	tview.Primitive
	UpdateKeybindings(string)
	UpdateVNCSessionCount(int)
	UpdateAutoRefreshStatus(bool)
	UpdateAutoRefreshCountdown(int)
	SetLoading(bool)
	IsLoading() bool
	TickSpinner()
}
