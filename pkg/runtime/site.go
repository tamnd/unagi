package runtime

import (
	"io"

	"github.com/tamnd/unagi/pkg/objects"
)

// The site builtins CPython installs from its site module: exit, quit,
// copyright, credits, license, and help. They are values, not fast-path
// calls, so lowering resolves each name to the object registered here and a
// call goes through objects.Call. copyright and credits carry the oracle's
// exact text; license and help carry the one-line prompt their repr shows and,
// as honest stubs, write that same line when called rather than paging the
// full license or opening an interactive session.
const (
	copyrightText = `Copyright (c) 2001 Python Software Foundation.
All Rights Reserved.

Copyright (c) 2000 BeOpen.com.
All Rights Reserved.

Copyright (c) 1995-2001 Corporation for National Research Initiatives.
All Rights Reserved.

Copyright (c) 1991-1995 Stichting Mathematisch Centrum, Amsterdam.
All Rights Reserved.`

	creditsText = `Thanks to CWI, CNRI, BeOpen, Zope Corporation, the Python Software
Foundation, and a cast of thousands for supporting Python
development.  See www.python.org for more information.`

	licenseText = "Type license() to see the full license text"

	helpText = "Type help() for interactive help, or help(object) for help about object."
)

func init() {
	register(map[string]objects.Object{
		"exit":      objects.NewQuitter("exit"),
		"quit":      objects.NewQuitter("quit"),
		"copyright": objects.NewPrinter("copyright", copyrightText, copyrightText),
		"credits":   objects.NewPrinter("credits", creditsText, creditsText),
		"license":   objects.NewPrinter("license", licenseText, licenseText),
		"help":      objects.NewHelper("help", helpText, helpText),
	})
	// A _Printer or _Helper writes through the same swappable sink print uses,
	// so a host that redirects Stdout captures copyright() too.
	objects.SetSiteWrite(func(s string) { _, _ = io.WriteString(Stdout, s) })
}
