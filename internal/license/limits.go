package license

import "fmt"

// FreeMoveBytes is how much the free version keeps moved at one time, unless
// only one app is moved (any size). Moving an app back gives the room back.
const FreeMoveBytes = 10 << 30

// MoveAllowed says whether a move of newBytes may start. movedCount and
// movedBytes describe the apps that are already moved. Plus and the trial have
// no limit. Free may always move one app; after that, moves must fit in
// FreeMoveBytes in total. Moving back is never limited and does not call this.
func MoveAllowed(tier Tier, movedCount int, movedBytes, newBytes uint64) (bool, string) {
	if tier == Plus || tier == Trial {
		return true, ""
	}
	if movedCount == 0 || movedBytes+newBytes <= FreeMoveBytes {
		return true, ""
	}
	return false, fmt.Sprintf(
		"The free version can keep one app moved, or up to %d GB in total. Move an app back to make room, or see Trimmel Plus in Settings for unlimited moves.",
		FreeMoveBytes>>30)
}
