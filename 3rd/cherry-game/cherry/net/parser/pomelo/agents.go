package pomelo

import (
	"sync"

	cerr "github.com/cherry-game/cherry/error"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
)

var (
	sidAgentMap = sync.Map{} // sid -> Agent
	uidMap      = sync.Map{} // uid -> sid
)

func BindSID(agent *Agent) {
	sidAgentMap.Store(agent.SID(), agent)
}

func Bind(sid cfacade.SID, uid cfacade.UID) (*Agent, error) {
	if sid == "" {
		return nil, cerr.Errorf("[sid = %s] less than 1.", sid)
	}

	if uid < 1 {
		return nil, cerr.Errorf("[uid = %d] less than 1.", uid)
	}

	agent, found := GetAgentWithSID(sid)
	if !found {
		return nil, cerr.Errorf("[sid = %s] does not exist.", sid)
	}

	var oldAgent *Agent
	if oldSID, found := GetSID(uid); found && oldSID != sid {
		if agent, exists := GetAgentWithSID(oldSID); exists {
			oldAgent = agent
		}
	}

	agent.session.Uid = uid
	uidMap.Store(uid, sid)

	return oldAgent, nil
}

func Unbind(sid cfacade.SID) {
	agent, found := GetAgentWithSIDAndDel(sid, true)
	if !found {
		return
	}

	if nowSID, ok := GetSID(agent.UID()); ok && nowSID == sid {
		uidMap.Delete(agent.UID())
	}

	clog.Debugf("Unbind agent. sid = %s", sid)
}

func GetAgentWithSIDAndDel(sid cfacade.SID, isDel bool) (*Agent, bool) {
	var (
		agentValue any
		found      bool
	)

	if isDel {
		agentValue, found = sidAgentMap.LoadAndDelete(sid)
	} else {
		agentValue, found = sidAgentMap.Load(sid)
	}

	if !found {
		return nil, false
	}

	agent, ok := agentValue.(*Agent)
	if !ok {
		return nil, false
	}

	return agent, found
}

func GetAgentWithSID(sid cfacade.SID) (*Agent, bool) {
	return GetAgentWithSIDAndDel(sid, false)
}

func GetAgentWithUID(uid cfacade.UID) (*Agent, bool) {
	if uid < 1 {
		return nil, false
	}

	sidValue, found := uidMap.Load(uid)
	if !found {
		return nil, false
	}

	sid := sidValue.(string)
	agentValue, found := sidAgentMap.Load(sid)
	if !found {
		return nil, false
	}

	agent, ok := agentValue.(*Agent)
	if !ok {
		return nil, false
	}

	return agent, found
}

func GetSID(uid int64) (cfacade.SID, bool) {
	sidValue, found := uidMap.Load(uid)
	if !found {
		return "", false
	}

	sid, ok := sidValue.(cfacade.SID)
	if !ok {
		return "", false
	}

	return sid, true
}

func GetAgent(sid string, uid cfacade.UID) (*Agent, bool) {
	if sid != "" {
		return GetAgentWithSID(sid)
	}

	if uid > 0 {
		return GetAgentWithUID(uid)
	}

	return nil, false
}

func ForeachAgent(fn func(a *Agent)) {
	sidAgentMap.Range(func(key, value any) bool {
		if agent, ok := value.(*Agent); ok {
			fn(agent)
		}
		return true
	})
}

func Count() int {
	count := 0
	sidAgentMap.Range(func(key, value any) bool {
		count += 1
		return true
	})

	return count
}
