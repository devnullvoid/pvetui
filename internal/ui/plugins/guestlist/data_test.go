package guestlist

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/devnullvoid/pvetui/pkg/api"
)

func TestFilterGuestRows(t *testing.T) {
	rows := []guestRow{
		{status: api.VMStatusRunning, filterHaystack: "101 web prod", id: 101},
		{status: api.VMStatusStopped, filterHaystack: "102 db prod", id: 102},
		{status: api.VMStatusRunning, filterHaystack: "103 cache", id: 103},
	}

	runningOnly := filterGuestRows(rows, false, "")
	require.Len(t, runningOnly, 2)
	require.Equal(t, 101, runningOnly[0].id)
	require.Equal(t, 103, runningOnly[1].id)

	withStopped := filterGuestRows(rows, true, "db")
	require.Len(t, withStopped, 1)
	require.Equal(t, 102, withStopped[0].id)
}

func TestSortGuestRows(t *testing.T) {
	base := []guestRow{
		{id: 101, cpuPercent: 10, memUsed: 1, memTotal: 2, uptime: 100, name: "web"},
		{id: 102, cpuPercent: 75, memUsed: 5, memTotal: 10, uptime: 10, name: "db"},
		{id: 103, cpuPercent: 33, memUsed: 7, memTotal: 8, uptime: 400, name: "cache"},
	}

	rows := append([]guestRow(nil), base...)
	sortGuestRows(rows, sortByCPU, true)
	require.Equal(t, []int{102, 103, 101}, []int{rows[0].id, rows[1].id, rows[2].id})

	rows = append([]guestRow(nil), base...)
	sortGuestRows(rows, sortByMemory, true)
	require.Equal(t, []int{103, 101, 102}, []int{rows[0].id, rows[1].id, rows[2].id})

	rows = append([]guestRow(nil), base...)
	sortGuestRows(rows, sortByUptime, false)
	require.Equal(t, []int{102, 101, 103}, []int{rows[0].id, rows[1].id, rows[2].id})

	rows = append([]guestRow(nil), base...)
	sortGuestRows(rows, sortByName, false)
	require.Equal(t, []int{103, 102, 101}, []int{rows[0].id, rows[1].id, rows[2].id})
}

func TestSummarizeGuests(t *testing.T) {
	rows := []guestRow{
		{status: api.VMStatusRunning, cpuPercent: 25, memUsed: 1, memTotal: 2},
		{status: api.VMStatusStopped, cpuPercent: 5, memUsed: 3, memTotal: 4},
	}

	visible := rows[:1]
	summary := summarizeGuests(rows, visible)
	require.Equal(t, 2, summary.totalGuests)
	require.Equal(t, 1, summary.visibleGuests)
	require.Equal(t, 1, summary.runningGuests)
	require.InDelta(t, 25.0, summary.cpuTotal, 0.01)
	require.EqualValues(t, 1, summary.memUsedTotal)
	require.EqualValues(t, 2, summary.memMaxTotal)
}

func TestNormalizeTags(t *testing.T) {
	require.Nil(t, normalizeTags(""))
	require.Equal(t, []string{"prod", "db"}, normalizeTags("prod;db"))
	require.Equal(t, []string{"alpha", "beta", "gamma"}, normalizeTags(" alpha, beta ; gamma "))
}
