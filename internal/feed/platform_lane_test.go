package feed

import "testing"

// TestWinLaneDriveTypeRefusal runs on every OS: the Windows rule is a pure
// function of the GetDriveTypeW answer.
func TestWinLaneDriveTypeRefusal(t *testing.T) {
	names := map[uint32]string{
		0: "DRIVE_UNKNOWN", 1: "DRIVE_NO_ROOT_DIR", 2: "DRIVE_REMOVABLE",
		3: "DRIVE_FIXED", 4: "DRIVE_REMOTE", 5: "DRIVE_CDROM", 6: "DRIVE_RAMDISK",
	}
	for kind := uint32(0); kind <= 6; kind++ {
		err := driveTypeRefusal(kind)
		if kind == 4 {
			if err == nil || err.Error() != "Feed requires a local filesystem" {
				t.Fatalf("%s: got %v, want the local filesystem refusal", names[kind], err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: got %v, want nil", names[kind], err)
		}
	}
	for _, kind := range []uint32{7, 8, 255, 1 << 31} {
		if err := driveTypeRefusal(kind); err != nil {
			t.Fatalf("drive type %d: got %v, want nil", kind, err)
		}
	}
}
