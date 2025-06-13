package undoer

func getShortHash(hash string) string {
	const lenShortHash = 8
	if len(hash) > lenShortHash {
		return hash[:lenShortHash]
	}
	return hash
}
