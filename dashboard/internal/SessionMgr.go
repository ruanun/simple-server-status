package internal

import (
	"github.com/olahol/melody"
	"sync"
)

// ws 会话信息存储
type SessionMgr struct {
	// key：服务器id 配置文件； value: session
	ServerIdSessionMap map[string]*melody.Session
	// key：session  value: 服务器id；
	SessionServerIdMap map[*melody.Session]string
	//lock
	SessionLock sync.RWMutex
}

var WsSessionMgr *SessionMgr

func (mgr *SessionMgr) GetServerId(session *melody.Session) (string, bool) {
	mgr.SessionLock.RLock()
	defer mgr.SessionLock.RUnlock()
	return mgr.SessionServerIdMap[session], mgr.SessionServerIdMap[session] != ""
}
func (mgr *SessionMgr) GetSession(serverId string) (*melody.Session, bool) {
	mgr.SessionLock.RLock()
	defer mgr.SessionLock.RUnlock()
	return mgr.ServerIdSessionMap[serverId], mgr.ServerIdSessionMap[serverId] != nil
}
func (mgr *SessionMgr) Add(serverId string, session *melody.Session) {
	mgr.SessionLock.Lock()
	defer mgr.SessionLock.Unlock()
	mgr.ServerIdSessionMap[serverId] = session
	mgr.SessionServerIdMap[session] = serverId
}
func (mgr *SessionMgr) DelByServerId(serverId string) {
	mgr.SessionLock.Lock()
	defer mgr.SessionLock.Unlock()
	session, ok := mgr.ServerIdSessionMap[serverId]
	if ok {
		delete(mgr.ServerIdSessionMap, serverId)
		delete(mgr.SessionServerIdMap, session)
	}
}

func (mgr *SessionMgr) DelBySession(session *melody.Session) {
	mgr.SessionLock.Lock()
	defer mgr.SessionLock.Unlock()
	serverId, ok := mgr.SessionServerIdMap[session]
	if ok {
		delete(mgr.ServerIdSessionMap, serverId)
		delete(mgr.SessionServerIdMap, session)
	}
}

func (mgr *SessionMgr) SessionLength() int {
	mgr.SessionLock.RLock()
	defer mgr.SessionLock.RUnlock()
	return len(mgr.SessionServerIdMap)
}

func (mgr *SessionMgr) ServerIdLength() int {
	mgr.SessionLock.RLock()
	defer mgr.SessionLock.RUnlock()
	return len(mgr.ServerIdSessionMap)
}

func (mgr *SessionMgr) GetAllServerId() []string {
	mgr.SessionLock.RLock()
	defer mgr.SessionLock.RUnlock()
	var serverIds []string
	for k := range mgr.ServerIdSessionMap {
		serverIds = append(serverIds, k)
	}
	return serverIds
}

func NewSessionMgr() *SessionMgr {
	return &SessionMgr{
		ServerIdSessionMap: make(map[string]*melody.Session),
		SessionServerIdMap: make(map[*melody.Session]string),
	}
}
