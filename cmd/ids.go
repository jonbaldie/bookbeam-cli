package cmd

import (
	"fmt"
	"strconv"
)

// resourceID parses a positional ID argument. Only the canonical form of a
// positive integer is accepted, so an argument can never reshape an API path.
func resourceID(arg, kind string) (int, error) {
	id, err := strconv.Atoi(arg)
	if err != nil || id < 1 || strconv.Itoa(id) != arg {
		return 0, fmt.Errorf("invalid %s id %q", kind, arg)
	}
	return id, nil
}

// projectAndChildIDs parses `<project-id> <child-id>` arguments, where child names the second ID's kind.
func projectAndChildIDs(args []string, child string) (int, int, error) {
	projectID, err := resourceID(args[0], "project")
	if err != nil {
		return 0, 0, err
	}
	childID, err := resourceID(args[1], child)
	if err != nil {
		return 0, 0, err
	}
	return projectID, childID, nil
}
