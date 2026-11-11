package feed

import "fmt"

// driveRemote is the DRIVE_REMOTE answer of the Windows GetDriveTypeW call: a
// network volume, including a mapped drive and a UNC share. The rule lives in a
// file every OS compiles, so it is tested without a Windows runtime.
const driveRemote = 4

// driveTypeRefusal refuses only a network volume, like the Linux and macOS
// twins. Every other drive type, including an unknown one, is accepted.
func driveTypeRefusal(driveType uint32) error {
	if driveType == driveRemote {
		return fmt.Errorf("Feed requires a local filesystem")
	}
	return nil
}
