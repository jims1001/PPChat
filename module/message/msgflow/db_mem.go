package msgflow

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrUniqueCID = errors.New("unique client_id violated")
	ErrUniqueSeq = errors.New("unique seq violated")
	ErrUniqueSID = errors.New("unique server_id violated")
)

type memDB struct {
	mu    sync.RWMutex
	convs map[string]struct{}           // tenant|conv
	bySeq map[string]map[int64]*Message // (tenant|conv) -> seq -> msg
	byCID map[string]*Message           // tenant|sender|cid -> msg
	bySID map[string]*Message           // sid -> msg
}

func NewMemDB() *memDB {
	return &memDB{
		convs: make(map[string]struct{}),
		bySeq: make(map[string]map[int64]*Message),
		byCID: make(map[string]*Message),
		bySID: make(map[string]*Message),
	}
}

func keyConv(tenant, conv string) string       { return tenant + "|" + conv }
func keyCID(tenant, sender, cid string) string { return tenant + "|" + sender + "|" + cid }

func (db *memDB) EnsureConversation(ctx context.Context, tenant, convID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.convs[keyConv(tenant, convID)] = struct{}{}
	return nil
}

func (db *memDB) QueryMaxSeq(ctx context.Context, tenant, convID string) (int64, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	k := keyConv(tenant, convID)
	m := db.bySeq[k]
	var max int64 = 0
	for s := range m {
		if s > max {
			max = s
		}
	}
	return max, nil
}

func (db *memDB) InsertMessage(ctx context.Context, m *Message) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// UNIQUE(server_msg_id)
	if _, ok := db.bySID[m.ServerMsgID]; ok {
		return ErrUniqueSID
	}
	// UNIQUE(tenant,sender,client_msg_id)
	kcid := keyCID(m.TenantID, m.SenderID, m.ClientMsgID)
	if _, ok := db.byCID[kcid]; ok {
		return ErrUniqueCID
	}
	// UNIQUE(tenant,conv,seq)
	kconv := keyConv(m.TenantID, m.ConvID)
	if _, ok := db.bySeq[kconv]; !ok {
		db.bySeq[kconv] = make(map[int64]*Message)
	}
	if _, ok := db.bySeq[kconv][m.Seq]; ok {
		return ErrUniqueSeq
	}

	cp := *m
	db.bySeq[kconv][m.Seq] = &cp
	db.byCID[kcid] = &cp
	db.bySID[m.ServerMsgID] = &cp
	return nil
}

func (db *memDB) FindByClientID(ctx context.Context, tenant, sender, clientMsgID string) (*MessageMeta, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if v, ok := db.byCID[keyCID(tenant, sender, clientMsgID)]; ok {
		return &MessageMeta{ServerMsgID: v.ServerMsgID, Seq: v.Seq, CreatedAtMS: v.CreatedAtMS}, nil
	}
	return nil, nil
}

func (db *memDB) FindBySeq(ctx context.Context, tenant, convID string, seq int64) (*MessageMeta, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	k := keyConv(tenant, convID)
	if mp, ok := db.bySeq[k]; ok {
		if v, ok2 := mp[seq]; ok2 {
			return &MessageMeta{ServerMsgID: v.ServerMsgID, Seq: v.Seq, CreatedAtMS: v.CreatedAtMS}, nil
		}
	}
	return nil, nil
}

func (db *memDB) FindByServerID(ctx context.Context, serverMsgID string) (*MessageMeta, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if v, ok := db.bySID[serverMsgID]; ok {
		return &MessageMeta{ServerMsgID: v.ServerMsgID, Seq: v.Seq, CreatedAtMS: v.CreatedAtMS}, nil
	}
	return nil, nil
}

func (db *memDB) IsUniqueClientIDErr(err error) bool { return errors.Is(err, ErrUniqueCID) }
func (db *memDB) IsUniqueSeqErr(err error) bool      { return errors.Is(err, ErrUniqueSeq) }
func (db *memDB) IsUniqueServerIDErr(err error) bool { return errors.Is(err, ErrUniqueSID) }
func (db *memDB) IsTransientErr(err error) bool      { return false } // 内存版无瞬时错误
