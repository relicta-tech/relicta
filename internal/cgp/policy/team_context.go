package policy

import (
	"strings"
)

// TeamContext provides team and role information for policy evaluation.
type TeamContext struct {
	// Teams maps team names to their definitions.
	Teams map[string]*Team

	// Roles maps role names to their permissions.
	Roles map[string]*Role

	// ActorTeams maps actor IDs to their team memberships.
	ActorTeams map[string][]string

	// ActorRoles maps actor IDs to their roles.
	ActorRoles map[string][]string
}

// Team represents a group of users with shared permissions.
type Team struct {
	// Name is the unique identifier for this team.
	Name string

	// Description explains the team's purpose.
	Description string

	// Members lists the actor IDs in this team.
	Members []string

	// Leads lists the actor IDs who are team leads.
	Leads []string

	// ParentTeam is the name of the parent team (for hierarchical teams).
	ParentTeam string

	// Permissions lists what this team is authorized to do.
	Permissions []string
}

// Role represents a set of permissions that can be assigned to actors.
type Role struct {
	// Name is the unique identifier for this role.
	Name string

	// Description explains the role's purpose.
	Description string

	// Permissions lists what this role is authorized to do.
	Permissions []string

	// CanApprove indicates if this role can approve releases.
	CanApprove bool

	// CanPublish indicates if this role can publish releases.
	CanPublish bool

	// RequiredForBreaking indicates if this role must approve breaking changes.
	RequiredForBreaking bool

	// RequiredForSecurity indicates if this role must approve security changes.
	RequiredForSecurity bool
}

// DefaultTeamContext creates an empty team context.
func DefaultTeamContext() *TeamContext {
	return &TeamContext{
		Teams:      make(map[string]*Team),
		Roles:      make(map[string]*Role),
		ActorTeams: make(map[string][]string),
		ActorRoles: make(map[string][]string),
	}
}

// NewTeamContext creates a team context with the given teams and roles.
func NewTeamContext(teams map[string]*Team, roles map[string]*Role) *TeamContext {
	tc := DefaultTeamContext()
	tc.Teams = teams
	tc.Roles = roles

	// Build actor mappings from team definitions
	for teamName, team := range teams {
		for _, member := range team.Members {
			tc.ActorTeams[member] = append(tc.ActorTeams[member], teamName)
		}
		for _, lead := range team.Leads {
			tc.ActorTeams[lead] = append(tc.ActorTeams[lead], teamName)
		}
	}

	return tc
}

// AddTeam adds a team to the context.
func (tc *TeamContext) AddTeam(team *Team) *TeamContext {
	tc.Teams[team.Name] = team

	// Update actor mappings
	for _, member := range team.Members {
		tc.ActorTeams[member] = append(tc.ActorTeams[member], team.Name)
	}
	for _, lead := range team.Leads {
		tc.ActorTeams[lead] = append(tc.ActorTeams[lead], team.Name)
	}

	return tc
}

// AddRole adds a role to the context.
func (tc *TeamContext) AddRole(role *Role) *TeamContext {
	tc.Roles[role.Name] = role
	return tc
}

// AssignRole assigns a role to an actor.
func (tc *TeamContext) AssignRole(actorID, roleName string) *TeamContext {
	tc.ActorRoles[actorID] = append(tc.ActorRoles[actorID], roleName)
	return tc
}

// GetTeam returns a team by name.
func (tc *TeamContext) GetTeam(name string) (*Team, bool) {
	team, ok := tc.Teams[name]
	return team, ok
}

// GetRole returns a role by name.
func (tc *TeamContext) GetRole(name string) (*Role, bool) {
	role, ok := tc.Roles[name]
	return role, ok
}

// GetActorTeams returns all teams an actor belongs to.
func (tc *TeamContext) GetActorTeams(actorID string) []string {
	return tc.ActorTeams[actorID]
}

// GetActorRoles returns all roles an actor has.
func (tc *TeamContext) GetActorRoles(actorID string) []string {
	return tc.ActorRoles[actorID]
}

// IsTeamMember checks if an actor is a member of a team.
func (tc *TeamContext) IsTeamMember(actorID, teamName string) bool {
	team, ok := tc.Teams[teamName]
	if !ok {
		return false
	}
	for _, member := range team.Members {
		if member == actorID {
			return true
		}
	}
	for _, lead := range team.Leads {
		if lead == actorID {
			return true
		}
	}
	return false
}

// IsTeamLead checks if an actor is a lead of a team.
func (tc *TeamContext) IsTeamLead(actorID, teamName string) bool {
	team, ok := tc.Teams[teamName]
	if !ok {
		return false
	}
	for _, lead := range team.Leads {
		if lead == actorID {
			return true
		}
	}
	return false
}

// HasRole checks if an actor has a specific role.
func (tc *TeamContext) HasRole(actorID, roleName string) bool {
	roles := tc.ActorRoles[actorID]
	for _, r := range roles {
		if r == roleName {
			return true
		}
	}
	return false
}

// HasPermission checks if an actor has a specific permission through any role or team.
func (tc *TeamContext) HasPermission(actorID, permission string) bool {
	// Check roles
	for _, roleName := range tc.ActorRoles[actorID] {
		if role, ok := tc.Roles[roleName]; ok {
			for _, perm := range role.Permissions {
				if perm == permission || matchesPermission(perm, permission) {
					return true
				}
			}
		}
	}

	// Check teams
	for _, teamName := range tc.ActorTeams[actorID] {
		if team, ok := tc.Teams[teamName]; ok {
			for _, perm := range team.Permissions {
				if perm == permission || matchesPermission(perm, permission) {
					return true
				}
			}
		}
	}

	return false
}

// CanApprove checks if an actor can approve releases.
func (tc *TeamContext) CanApprove(actorID string) bool {
	for _, roleName := range tc.ActorRoles[actorID] {
		if role, ok := tc.Roles[roleName]; ok {
			if role.CanApprove {
				return true
			}
		}
	}
	return false
}

// CanPublish checks if an actor can publish releases.
func (tc *TeamContext) CanPublish(actorID string) bool {
	for _, roleName := range tc.ActorRoles[actorID] {
		if role, ok := tc.Roles[roleName]; ok {
			if role.CanPublish {
				return true
			}
		}
	}
	return false
}

// GetRequiredApproversForBreaking returns actor IDs required to approve breaking changes.
func (tc *TeamContext) GetRequiredApproversForBreaking() []string {
	var approvers []string
	for actorID, roleNames := range tc.ActorRoles {
		for _, roleName := range roleNames {
			if role, ok := tc.Roles[roleName]; ok {
				if role.RequiredForBreaking {
					approvers = append(approvers, actorID)
					break
				}
			}
		}
	}
	return approvers
}

// GetRequiredApproversForSecurity returns actor IDs required to approve security changes.
func (tc *TeamContext) GetRequiredApproversForSecurity() []string {
	var approvers []string
	for actorID, roleNames := range tc.ActorRoles {
		for _, roleName := range roleNames {
			if role, ok := tc.Roles[roleName]; ok {
				if role.RequiredForSecurity {
					approvers = append(approvers, actorID)
					break
				}
			}
		}
	}
	return approvers
}

// GetTeamMembers returns all members of a team (including leads).
func (tc *TeamContext) GetTeamMembers(teamName string) []string {
	team, ok := tc.Teams[teamName]
	if !ok {
		return nil
	}

	// Use a map to deduplicate
	members := make(map[string]bool)
	for _, m := range team.Members {
		members[m] = true
	}
	for _, l := range team.Leads {
		members[l] = true
	}

	result := make([]string, 0, len(members))
	for m := range members {
		result = append(result, m)
	}
	return result
}

// GetTeamLeads returns the leads of a team.
func (tc *TeamContext) GetTeamLeads(teamName string) []string {
	team, ok := tc.Teams[teamName]
	if !ok {
		return nil
	}
	return team.Leads
}

// ToEvalContext converts the team context to a map for policy evaluation.
func (tc *TeamContext) ToEvalContext(actorID string) map[string]any {
	// Build team list with details
	teamsCtx := make(map[string]any)
	for name, team := range tc.Teams {
		teamsCtx[name] = map[string]any{
			"name":        team.Name,
			"description": team.Description,
			"memberCount": len(team.Members) + len(team.Leads),
			"members":     team.Members,
			"leads":       team.Leads,
			"permissions": team.Permissions,
		}
	}

	// Build role list with details
	rolesCtx := make(map[string]any)
	for name, role := range tc.Roles {
		rolesCtx[name] = map[string]any{
			"name":                role.Name,
			"description":         role.Description,
			"canApprove":          role.CanApprove,
			"canPublish":          role.CanPublish,
			"requiredForBreaking": role.RequiredForBreaking,
			"requiredForSecurity": role.RequiredForSecurity,
			"permissions":         role.Permissions,
		}
	}

	// Build actor context
	actorTeams := tc.GetActorTeams(actorID)
	actorRoles := tc.GetActorRoles(actorID)

	return map[string]any{
		"teams":      teamsCtx,
		"roles":      rolesCtx,
		"actorTeams": actorTeams,
		"actorRoles": actorRoles,
		"canApprove": tc.CanApprove(actorID),
		"canPublish": tc.CanPublish(actorID),
		"isTeamLead": tc.isAnyTeamLead(actorID),
		"teamCount":  len(actorTeams),
		"roleCount":  len(actorRoles),
	}
}

// isAnyTeamLead checks if an actor is a lead of any team.
func (tc *TeamContext) isAnyTeamLead(actorID string) bool {
	for _, team := range tc.Teams {
		for _, lead := range team.Leads {
			if lead == actorID {
				return true
			}
		}
	}
	return false
}

// matchesPermission checks if a pattern matches a permission.
// Supports wildcard patterns like "release.*" matching "release.approve".
func matchesPermission(pattern, permission string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(permission, prefix+".")
	}
	return pattern == permission
}

// Common role definitions for convenience.

// NewApproverRole creates a role with approval permissions.
func NewApproverRole(name, description string) *Role {
	return &Role{
		Name:        name,
		Description: description,
		CanApprove:  true,
		Permissions: []string{"release.approve"},
	}
}

// NewPublisherRole creates a role with publish permissions.
func NewPublisherRole(name, description string) *Role {
	return &Role{
		Name:        name,
		Description: description,
		CanApprove:  true,
		CanPublish:  true,
		Permissions: []string{"release.approve", "release.publish"},
	}
}

// NewSecurityReviewerRole creates a role required for security changes.
func NewSecurityReviewerRole(name, description string) *Role {
	return &Role{
		Name:                name,
		Description:         description,
		CanApprove:          true,
		RequiredForSecurity: true,
		Permissions:         []string{"release.approve", "security.review"},
	}
}

// NewArchitectRole creates a role required for breaking changes.
func NewArchitectRole(name, description string) *Role {
	return &Role{
		Name:                name,
		Description:         description,
		CanApprove:          true,
		RequiredForBreaking: true,
		Permissions:         []string{"release.approve", "architecture.review"},
	}
}

// NewTeam creates a new team with the given name.
func NewTeam(name, description string) *Team {
	return &Team{
		Name:        name,
		Description: description,
		Members:     []string{},
		Leads:       []string{},
		Permissions: []string{},
	}
}

// WithMembers adds members to the team.
func (t *Team) WithMembers(members ...string) *Team {
	t.Members = append(t.Members, members...)
	return t
}

// WithLeads adds leads to the team.
func (t *Team) WithLeads(leads ...string) *Team {
	t.Leads = append(t.Leads, leads...)
	return t
}

// WithPermissions adds permissions to the team.
func (t *Team) WithPermissions(permissions ...string) *Team {
	t.Permissions = append(t.Permissions, permissions...)
	return t
}

// WithParent sets the parent team.
func (t *Team) WithParent(parentName string) *Team {
	t.ParentTeam = parentName
	return t
}
