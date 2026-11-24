package update

// Linux permits unlinking the old executable while the old process finishes exiting.
// The inherited pipe already ensures its server and data roots have closed.
func waitForPreviousProcess(_ int) error {
	return nil
}
