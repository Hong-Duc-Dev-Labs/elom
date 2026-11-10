// Package core holds the parts of Elom that know nothing of the user
// interface: the safety rules, the cleanup catalog, scanning, deletion,
// installed apps, startup entries, metrics and maintenance tasks. Packages
// under core never import MyGo, so the same code can serve a command line
// tool (see arch_test.go).
package core
