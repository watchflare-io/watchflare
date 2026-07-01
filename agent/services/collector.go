package services

import (
	"sort"
	"strings"
)

// Service is a tracked systemd service for the inventory.
type Service struct {
	Name         string
	Description  string
	EnabledState string
	ActiveState  string
	SubState     string
}

// ServiceHealth is the volatile state reported every 30s.
type ServiceHealth struct {
	Name        string
	ActiveState string
	SubState    string
}

type rawUnit struct {
	Name        string
	Description string
	ActiveState string
	SubState    string
}

type rawUnitFile struct {
	Name  string
	State string
}

// mergeServices keeps the union of enabled and active .service units.
func mergeServices(units []rawUnit, files []rawUnitFile) []*Service {
	enabled := make(map[string]string, len(files))
	for _, f := range files {
		enabled[f.Name] = f.State
	}
	unitByName := make(map[string]rawUnit, len(units))
	for _, u := range units {
		unitByName[u.Name] = u
	}

	keep := make(map[string]struct{})
	for name, state := range enabled {
		if state == "enabled" && strings.HasSuffix(name, ".service") {
			keep[name] = struct{}{}
		}
	}
	for _, u := range units {
		if u.ActiveState == "active" {
			keep[u.Name] = struct{}{}
		}
	}

	result := make([]*Service, 0, len(keep))
	for name := range keep {
		svc := &Service{Name: name, EnabledState: enabled[name]}
		if u, ok := unitByName[name]; ok {
			svc.Description = u.Description
			svc.ActiveState = u.ActiveState
			svc.SubState = u.SubState
		} else {
			svc.ActiveState = "inactive"
			svc.SubState = "dead"
		}
		result = append(result, svc)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
