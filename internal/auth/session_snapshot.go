package auth

func snapshotSessions(sessions []Session) []Session {
	result := make([]Session, len(sessions))
	copy(result, sessions)
	return result
}
