package startup

import "strings"

// Advice is a plain-English note about a startup entry: what it is, how much
// it slows sign-in, and whether turning it off is usually fine. Impact is a
// rough guide from what the program typically does, not a measurement.
type Advice struct {
	Impact    string // "high", "medium" or "low"
	Note      string
	Recommend string // "disable", "keep" or "" for no opinion
}

type rule struct {
	match []string // lower-case substrings of the entry name or command
	Advice
}

// Order matters: the first matching rule wins, so specific names come first.
var rules = []rule{
	{[]string{"securityhealth", "windows defender", "msmpeng"}, Advice{"low", "Windows Security. Keep it on.", "keep"}},
	{[]string{"avast", "avg", "mcafee", "norton", "kaspersky", "bitdefender", "malwarebytes", "eset"}, Advice{"medium", "Antivirus protection. Keep it on, and use only one antivirus.", "keep"}},
	{[]string{"rtkaud", "realtek", "waves", "nahimic", "soundblaster"}, Advice{"low", "Sound card helper. Disabling it can break audio controls or effects.", "keep"}},
	{[]string{"nvidia", "nvcontainer", "nvbackend", "igfx", "intel(r) graphics", "radeon"}, Advice{"low", "Graphics driver helper. Keep it if you use its control panel or overlay.", "keep"}},
	{[]string{"onedrive"}, Advice{"medium", "Syncs files to Microsoft OneDrive. Disable it if you don't use OneDrive.", ""}},
	{[]string{"teams"}, Advice{"high", "Microsoft Teams chat. It opens at sign-in and uses a lot of memory. Open it yourself when you need it.", "disable"}},
	{[]string{"skype"}, Advice{"high", "Skype. Open it yourself when you need it.", "disable"}},
	{[]string{"zoom"}, Advice{"medium", "Zoom meetings. Meeting links still start it when needed.", "disable"}},
	{[]string{"webex", "gotomeeting", "slack"}, Advice{"medium", "Chat or meeting app. It starts fine when you open it.", "disable"}},
	{[]string{"spotify"}, Advice{"medium", "Spotify music player. Open it yourself when you want music.", "disable"}},
	{[]string{"discord"}, Advice{"high", "Discord chat. Keep it only if you want to be online as soon as you sign in.", "disable"}},
	{[]string{"steam"}, Advice{"high", "Steam game launcher. Games still start it when needed.", "disable"}},
	{[]string{"epicgames", "epic games", "battle.net", "eadm", "ubisoft", "uplay", "gog galaxy", "riot"}, Advice{"medium", "Game launcher. It starts when you launch a game.", "disable"}},
	{[]string{"adobeaamupdater", "adobegcinvoker", "adobe reader synchronizer", "acrotray", "adobe creative cloud", "ccxprocess", "adobeupdate"}, Advice{"medium", "Adobe update and background helpers. Safe to disable; Adobe apps still work and update when opened.", "disable"}},
	{[]string{"googleupdate", "google chrome", "chrome"}, Advice{"low", "Keeps Chrome up to date. Safe to disable; Chrome updates when you open it.", "disable"}},
	{[]string{"microsoftedgeupdate", "msedge"}, Advice{"low", "Keeps Edge up to date.", ""}},
	{[]string{"itunes", "ipodservice", "apple push"}, Advice{"medium", "Apple device helper. Only needed to sync an iPhone or iPod.", "disable"}},
	{[]string{"java", "jusched", "jucheck"}, Advice{"low", "Java update checker. Safe to disable.", "disable"}},
	{[]string{"dropbox", "box sync", "google drive", "googledrivefs"}, Advice{"medium", "Cloud file sync. Keep it if you rely on automatic syncing.", ""}},
	{[]string{"logi", "lghub", "synapse", "razer", "corsair", "icue", "steelseries"}, Advice{"medium", "Keyboard, mouse or lighting software. Keep it if you use custom buttons or lighting profiles.", ""}},
	{[]string{"ccleaner", "driverupdui", "driver booster", "iobit", "web companion", "opera browser assistant", "yandex"}, Advice{"medium", "Optional helper or promotional tool. Usually safe to disable.", "disable"}},
	{[]string{"cortana", "yourphone", "phone link", "widgets", "gamebar"}, Advice{"low", "Optional Windows feature. Safe to disable if you don't use it.", "disable"}},
}

func Advise(name, command string) Advice {
	hay := strings.ToLower(name + " " + exeName(command))
	for _, r := range rules {
		for _, m := range r.match {
			if strings.Contains(hay, m) {
				return r.Advice
			}
		}
	}
	return Advice{Impact: "low"}
}

// exeName pulls the program's file name out of a command line, so a match
// can't come from a folder name such as "...\Original Files\".
func exeName(cmd string) string {
	l := strings.ToLower(cmd)
	i := strings.Index(l, ".exe")
	if i < 0 {
		return ""
	}
	l = l[:i+4]
	if j := strings.LastIndexAny(l, `\/"`); j >= 0 {
		l = l[j+1:]
	}
	return l
}
