package imap

import (
	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// mockImapClient is a test double for imapClient.
// Zero value is usable: all operations succeed and return
// empty/nil results.  Override individual fields to control
// behavior.
type mockImapClient struct {
	// Inject errors for specific commands.
	selectErr     error
	storeErr      error
	moveErr       error
	copyErr       error
	expungeErr    error
	appendErr     error
	appendWaitErr error
	listErr       error
	statusErr     error
	createErr     error
	deleteErr     error
	fetchErr      error
	searchErr     error

	// Recorded call arguments (for assertion in tests).
	selectCalls  []selectCall
	storeCalls   []storeCall
	moveCalls    []transferCall
	copyCalls    []transferCall
	expungeCalls []expungeCall
	appendCalls  []appendCall
	listCalls    int
	createCalls  []string
	deleteCalls  []string

	// Return values for specific commands.
	listData   []*imap.ListData
	statusData *imap.StatusData
	searchData *imap.SearchData
	fetchData  []*imapclient.FetchMessageBuffer
	selectData *imap.SelectData

	// closedCh is never closed, so the mock looks alive.
	closedCh chan struct{}
}

type selectCall struct {
	mailbox  string
	readOnly bool
}

type storeCall struct {
	uidSet imap.UIDSet
	op     imap.StoreFlagsOp
	flags  []imap.Flag
}

type transferCall struct {
	mailbox     string
	uidSet      imap.UIDSet
	destMailbox string
}

type expungeCall struct {
	mailbox string
	uidSet  imap.UIDSet
}

type appendCall struct {
	mailbox string
	data    []byte
	flags   []imap.Flag
}

func newMockImapClient() *mockImapClient {
	return &mockImapClient{
		closedCh: make(chan struct{}),
	}
}

// injectMock stores m as the connection for account in mgr.
func injectMock(
	mgr *ConnectionManager,
	account string,
	m *mockImapClient,
) {
	mgr.mu.Lock()
	mgr.conns[account] = m
	mgr.mu.Unlock()
}

// --- imapClient implementation ---

func (m *mockImapClient) Closed() <-chan struct{} {
	return m.closedCh
}

func (m *mockImapClient) Close() error { return nil }

func (m *mockImapClient) Select(
	mailbox string,
	options *imap.SelectOptions,
) selectResult {
	readOnly := options != nil && options.ReadOnly
	m.selectCalls = append(m.selectCalls, selectCall{
		mailbox:  mailbox,
		readOnly: readOnly,
	})
	data := m.selectData
	if data == nil {
		data = &imap.SelectData{}
	}
	return &mockSelectResult{data: data, err: m.selectErr}
}

func (m *mockImapClient) Store(
	numSet imap.NumSet,
	store *imap.StoreFlags,
	_ *imap.StoreOptions,
) storeResult {
	if uidSet, ok := numSet.(imap.UIDSet); ok {
		m.storeCalls = append(m.storeCalls, storeCall{
			uidSet: uidSet,
			op:     store.Op,
			flags:  store.Flags,
		})
	}
	return &mockStoreResult{err: m.storeErr}
}

func (m *mockImapClient) Move(
	numSet imap.NumSet,
	destMailbox string,
) moveResult {
	if uidSet, ok := numSet.(imap.UIDSet); ok {
		m.moveCalls = append(m.moveCalls, transferCall{
			uidSet:      uidSet,
			destMailbox: destMailbox,
		})
	}
	return &mockMoveResult{err: m.moveErr}
}

func (m *mockImapClient) Copy(
	numSet imap.NumSet,
	destMailbox string,
) copyResult {
	if uidSet, ok := numSet.(imap.UIDSet); ok {
		m.copyCalls = append(m.copyCalls, transferCall{
			uidSet:      uidSet,
			destMailbox: destMailbox,
		})
	}
	return &mockCopyResult{err: m.copyErr}
}

func (m *mockImapClient) UIDExpunge(
	uids imap.UIDSet,
) expungeResult {
	m.expungeCalls = append(m.expungeCalls, expungeCall{
		uidSet: uids,
	})
	return &mockExpungeResult{err: m.expungeErr}
}

func (m *mockImapClient) Append(
	mailbox string,
	_ int64,
	options *imap.AppendOptions,
) appendResult {
	var flags []imap.Flag
	if options != nil {
		flags = options.Flags
	}
	return &mockAppendResult{
		mock:    m,
		mailbox: mailbox,
		flags:   flags,
	}
}

func (m *mockImapClient) List(
	_, _ string,
	_ *imap.ListOptions,
) listResult {
	m.listCalls++
	return &mockListResult{
		data: m.listData,
		err:  m.listErr,
	}
}

func (m *mockImapClient) Status(
	_ string,
	_ *imap.StatusOptions,
) statusResult {
	data := m.statusData
	if data == nil {
		data = &imap.StatusData{}
	}
	return &mockStatusResult{data: data, err: m.statusErr}
}

func (m *mockImapClient) Create(
	mailbox string,
	_ *imap.CreateOptions,
) commandResult {
	m.createCalls = append(m.createCalls, mailbox)
	return &mockCommandResult{err: m.createErr}
}

func (m *mockImapClient) Delete(mailbox string) commandResult {
	m.deleteCalls = append(m.deleteCalls, mailbox)
	return &mockCommandResult{err: m.deleteErr}
}

func (m *mockImapClient) Fetch(
	_ imap.NumSet,
	_ *imap.FetchOptions,
) fetchResult {
	return &mockFetchResult{
		data: m.fetchData,
		err:  m.fetchErr,
	}
}

func (m *mockImapClient) UIDSearch(
	_ *imap.SearchCriteria,
	_ *imap.SearchOptions,
) searchResult {
	data := m.searchData
	if data == nil {
		data = &imap.SearchData{}
	}
	return &mockSearchResult{data: data, err: m.searchErr}
}

// --- mock command result types ---

type mockSelectResult struct {
	data *imap.SelectData
	err  error
}

func (r *mockSelectResult) Wait() (*imap.SelectData, error) {
	return r.data, r.err
}

type mockStoreResult struct{ err error }

func (r *mockStoreResult) Close() error { return r.err }

type mockMoveResult struct{ err error }

func (r *mockMoveResult) Wait() (*imapclient.MoveData, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &imapclient.MoveData{}, nil
}

type mockCopyResult struct{ err error }

func (r *mockCopyResult) Wait() (*imap.CopyData, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &imap.CopyData{}, nil
}

type mockExpungeResult struct{ err error }

func (r *mockExpungeResult) Close() error { return r.err }

// mockAppendResult accumulates written bytes and records the
// call on the parent mock once Close is called.
type mockAppendResult struct {
	mock    *mockImapClient
	mailbox string
	flags   []imap.Flag
	buf     []byte
	closed  bool
}

func (r *mockAppendResult) Write(b []byte) (int, error) {
	if r.mock.appendErr != nil {
		return 0, r.mock.appendErr
	}
	r.buf = append(r.buf, b...)
	return len(b), nil
}

func (r *mockAppendResult) Close() error {
	if r.mock.appendErr != nil {
		return r.mock.appendErr
	}
	r.closed = true
	return nil
}

func (r *mockAppendResult) Wait() (*imap.AppendData, error) {
	if r.mock.appendWaitErr != nil {
		return nil, r.mock.appendWaitErr
	}
	// Record the call now that all data has been written.
	r.mock.appendCalls = append(r.mock.appendCalls, appendCall{
		mailbox: r.mailbox,
		data:    r.buf,
		flags:   r.flags,
	})
	return &imap.AppendData{}, nil
}

type mockListResult struct {
	data []*imap.ListData
	err  error
}

func (r *mockListResult) Collect() ([]*imap.ListData, error) {
	return r.data, r.err
}

type mockStatusResult struct {
	data *imap.StatusData
	err  error
}

func (r *mockStatusResult) Wait() (*imap.StatusData, error) {
	return r.data, r.err
}

type mockCommandResult struct{ err error }

func (r *mockCommandResult) Wait() error { return r.err }

type mockFetchResult struct {
	data []*imapclient.FetchMessageBuffer
	err  error
}

func (r *mockFetchResult) Collect() (
	[]*imapclient.FetchMessageBuffer,
	error,
) {
	return r.data, r.err
}

type mockSearchResult struct {
	data *imap.SearchData
	err  error
}

func (r *mockSearchResult) Wait() (*imap.SearchData, error) {
	return r.data, r.err
}
