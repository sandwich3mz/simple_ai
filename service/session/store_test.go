package session

func replaceSessionStoreForTest(store sessionStoreFuncs) func() {
	oldStore := sessionStore
	if store.createSession == nil {
		store.createSession = oldStore.createSession
	}
	if store.listByUserName == nil {
		store.listByUserName = oldStore.listByUserName
	}
	if store.sessionBelongsToUser == nil {
		store.sessionBelongsToUser = oldStore.sessionBelongsToUser
	}
	if store.messagesBySession == nil {
		store.messagesBySession = oldStore.messagesBySession
	}
	sessionStore = store

	return func() {
		sessionStore = oldStore
	}
}
