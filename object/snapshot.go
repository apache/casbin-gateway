// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"errors"
	"fmt"
	"sort"

	"github.com/apache/casbin-gateway/connector"
	"github.com/apache/casbin-gateway/util"
	"github.com/apache/casbin-gateway/version"
)

// SnapshotVersion is the format of the document below. A snapshot written by a
// later Gateway is refused rather than half-read.
const SnapshotVersion = 1

// The sections a snapshot is made of. They are named rather than positional
// because a snapshot is often taken, and restored, one section at a time.
const (
	SectionProviders   = "providers"
	SectionConnections = "connections"
	SectionAgents      = "agents"
	SectionProbeCases  = "probeCases"
	SectionLlmPrices   = "llmPrices"
	SectionSetting     = "setting"
)

// How an import treats a row that is already there.
const (
	// ImportMerge leaves every existing row alone and adds the rest, which is
	// what merging two machines' configurations means.
	ImportMerge = "merge"
	// ImportOverwrite rewrites the rows the snapshot names and leaves the rest.
	ImportOverwrite = "overwrite"
	// ImportReplace additionally deletes what the snapshot does not name, so the
	// sections it covers end up exactly as they were when it was taken. This is
	// what restoring a backup means.
	ImportReplace = "replace"
)

// What one row of an import did, for the report the page shows before and after.
const (
	ChangeAdd    = "add"
	ChangeUpdate = "update"
	ChangeDelete = "delete"
	ChangeSkip   = "skip"
	ChangeFail   = "fail"
)

// SnapshotScope is which sections a snapshot carries, and whether the
// credentials inside them come with it.
type SnapshotScope struct {
	Providers bool `json:"providers"`
	// Connections are the applications this machine is connected to. They are
	// their own section rather than part of the agents, because a connection
	// belongs to the account and only names the agents it was written into.
	Connections bool `json:"connections"`
	Agents      bool `json:"agents"`
	ProbeCases  bool `json:"probeCases"`
	LlmPrices   bool `json:"llmPrices"`
	Setting     bool `json:"setting"`
	// Secrets carries the provider API keys, the credentials of every
	// connection, and the secret half of the settings.
	// A snapshot without them describes a configuration that cannot yet serve
	// traffic: every key has to be typed again on the machine it lands on.
	Secrets bool `json:"secrets"`
}

// FullScope is everything, which is what a backup takes: a backup that leaves
// the keys out is not one anybody can restore from.
func FullScope() SnapshotScope {
	return SnapshotScope{
		Providers: true, Connections: true, Agents: true,
		ProbeCases: true, LlmPrices: true, Setting: true, Secrets: true,
	}
}

func (scope SnapshotScope) isEmpty() bool {
	return !scope.Providers && !scope.Connections && !scope.Agents &&
		!scope.ProbeCases && !scope.LlmPrices && !scope.Setting
}

// Snapshot is the whole document, and the file an export downloads. It holds
// the configuration only: the records, the probe reports and the usage history
// describe what happened on one machine rather than how it is set up, and they
// are what makes a database too big to move around.
type Snapshot struct {
	Version     int    `json:"version"`
	CreatedTime string `json:"createdTime"`
	// Gateway and Host say where the snapshot came from, which is the first
	// thing anybody restoring one wants to know.
	Gateway string `json:"gateway"`
	Host    string `json:"host"`
	// Reason is what took it: "manual", "schedule", or what an automatic
	// snapshot was taken in front of.
	Reason  string        `json:"reason"`
	Scope   SnapshotScope `json:"scope"`
	Setting *Setting      `json:"setting,omitempty"`

	Providers      []*Provider      `json:"providers,omitempty"`
	Connections    []*Connection    `json:"connections,omitempty"`
	Agents         []*Agent         `json:"agents,omitempty"`
	AgentInstances []*AgentInstance `json:"agentInstances,omitempty"`
	// AgentPermissions travel with the agents: what an agent is allowed to ask
	// for is part of how this machine is set up, not of what happened on it.
	AgentPermissions []*AgentPermission `json:"agentPermissions,omitempty"`
	ProbeCases       []*ProbeCase       `json:"probeCases,omitempty"`
	LlmPrices        []*LlmPriceEntry   `json:"llmPrices,omitempty"`
}

// SnapshotCounts is how much a snapshot holds, which is what a listing shows
// without reading the whole file back to the browser.
type SnapshotCounts struct {
	Providers      int `json:"providers"`
	Connections    int `json:"connections"`
	Agents         int `json:"agents"`
	AgentInstances int `json:"agentInstances"`
	ProbeCases     int `json:"probeCases"`
	LlmPrices      int `json:"llmPrices"`
	Setting        int `json:"setting"`
}

func (snapshot *Snapshot) Counts() SnapshotCounts {
	counts := SnapshotCounts{
		Providers:      len(snapshot.Providers),
		Connections:    len(snapshot.Connections),
		Agents:         len(snapshot.Agents),
		AgentInstances: len(snapshot.AgentInstances),
		ProbeCases:     len(snapshot.ProbeCases),
		LlmPrices:      len(snapshot.LlmPrices),
	}
	if snapshot.Setting != nil {
		counts.Setting = 1
	}
	return counts
}

// ImportChange is one row an import touched, or refused to.
type ImportChange struct {
	Section string `json:"section"`
	Id      string `json:"id"`
	Action  string `json:"action"`
	// Detail is why a row was skipped or how it failed. Empty otherwise.
	Detail string `json:"detail"`
}

// ImportReport is what an import did, and what a dry run says it would do. The
// two are the same document so the confirmation and the result read alike.
type ImportReport struct {
	DryRun  bool            `json:"dryRun"`
	Mode    string          `json:"mode"`
	Added   int             `json:"added"`
	Updated int             `json:"updated"`
	Deleted int             `json:"deleted"`
	Skipped int             `json:"skipped"`
	Failed  int             `json:"failed"`
	Changes []*ImportChange `json:"changes"`
	// Backup is the snapshot taken before anything was written, empty on a dry
	// run and when the backup itself could not be taken.
	Backup string `json:"backup"`
}

func (report *ImportReport) record(section string, id string, action string, detail string) {
	report.Changes = append(report.Changes, &ImportChange{Section: section, Id: id, Action: action, Detail: detail})
	switch action {
	case ChangeAdd:
		report.Added++
	case ChangeUpdate:
		report.Updated++
	case ChangeDelete:
		report.Deleted++
	case ChangeSkip:
		report.Skipped++
	case ChangeFail:
		report.Failed++
	}
}

// BuildSnapshot reads the configuration out of the database. Provider keys come
// back decrypted, so the snapshot is readable on a machine whose encryption key
// is a different one - or none at all.
func BuildSnapshot(scope SnapshotScope, reason string) (*Snapshot, error) {
	if scope.isEmpty() {
		return nil, errors.New("a snapshot has to carry at least one section")
	}

	snapshot := &Snapshot{
		Version:     SnapshotVersion,
		CreatedTime: util.GetCurrentTime(),
		Gateway:     version.Current().Version,
		Host:        util.GetHostname(),
		Reason:      reason,
		Scope:       scope,
	}

	if scope.Providers {
		providers, err := GetProviders("")
		if err != nil {
			return nil, err
		}
		snapshot.Providers = providers
	}

	if scope.Connections {
		connections, err := getAllConnections()
		if err != nil {
			return nil, err
		}
		snapshot.Connections = connections
	}

	if scope.Agents {
		agents, err := GetAgents()
		if err != nil {
			return nil, err
		}
		for _, agent := range agents {
			snapshot.Agents = append(snapshot.Agents, agent)
		}
		sort.Slice(snapshot.Agents, func(i, j int) bool { return snapshot.Agents[i].Name < snapshot.Agents[j].Name })

		instances, err := GetAgentInstances("")
		if err != nil {
			return nil, err
		}
		snapshot.AgentInstances = instances

		permissions, err := GetAgentPermissions()
		if err != nil {
			return nil, err
		}
		for _, permission := range permissions {
			snapshot.AgentPermissions = append(snapshot.AgentPermissions, permission)
		}
		sort.Slice(snapshot.AgentPermissions, func(i, j int) bool {
			return snapshot.AgentPermissions[i].Name < snapshot.AgentPermissions[j].Name
		})
	}

	if scope.ProbeCases {
		cases, err := GetProbeCases()
		if err != nil {
			return nil, err
		}
		snapshot.ProbeCases = cases
	}

	if scope.LlmPrices {
		prices, err := GetLlmPriceEntries()
		if err != nil {
			return nil, err
		}
		snapshot.LlmPrices = prices
	}

	if scope.Setting {
		setting, err := GetBuiltInSetting()
		if err != nil {
			return nil, err
		}
		snapshot.Setting = setting
	}

	if !scope.Secrets {
		redactSnapshot(snapshot)
	}
	return snapshot, nil
}

// redactSnapshot empties every credential in place. The rows are copies read
// for this snapshot alone, so nothing stored is touched.
func redactSnapshot(snapshot *Snapshot) {
	for _, provider := range snapshot.Providers {
		provider.ApiKey = ""
		if provider.Quota != nil {
			provider.Quota.Token = ""
		}
	}

	for _, connection := range snapshot.Connections {
		found, ok := connector.Get(connection.Name)
		if !ok {
			continue
		}
		for _, key := range found.SecretKeys() {
			delete(connection.Credentials, key)
		}
	}

	if snapshot.Setting != nil {
		snapshot.Setting.ApiKeyEncryptionKey = ""
		snapshot.Setting.ClientSecret = ""
		snapshot.Setting.RelayToken = ""
	}
}

// ImportSnapshot writes a snapshot back. Only the sections the caller asks for
// are touched, and only the ones the snapshot actually carries: importing the
// providers out of a full backup leaves everything else where it is.
//
// A dry run walks the same path and decides the same things without writing,
// which is what the page shows before anybody presses the button.
func ImportSnapshot(snapshot *Snapshot, scope SnapshotScope, mode string, dryRun bool) (*ImportReport, error) {
	if snapshot == nil {
		return nil, errors.New("there is no snapshot to import")
	}
	if snapshot.Version > SnapshotVersion {
		return nil, fmt.Errorf("this snapshot was written by a later Gateway (format %d, this one reads %d)", snapshot.Version, SnapshotVersion)
	}
	switch mode {
	case ImportMerge, ImportOverwrite, ImportReplace:
	default:
		return nil, fmt.Errorf("the import mode must be %q, %q or %q", ImportMerge, ImportOverwrite, ImportReplace)
	}
	if scope.isEmpty() {
		return nil, errors.New("an import has to cover at least one section")
	}

	report := &ImportReport{DryRun: dryRun, Mode: mode, Changes: []*ImportChange{}}

	if scope.Providers && snapshot.Scope.Providers {
		if err := importProviders(snapshot, mode, dryRun, report); err != nil {
			return nil, err
		}
	}
	// Agent routing names providers, so it is written after them: a routing that
	// arrives with its provider in the same snapshot resolves on the first pass.
	if scope.Agents && snapshot.Scope.Agents {
		if err := importAgents(snapshot, mode, dryRun, report); err != nil {
			return nil, err
		}
	}
	// Connections name the agents they were written into, so they follow them.
	if scope.Connections && snapshot.Scope.Connections {
		if err := importConnections(snapshot, mode, dryRun, report); err != nil {
			return nil, err
		}
	}
	if scope.ProbeCases && snapshot.Scope.ProbeCases {
		if err := importProbeCases(snapshot, mode, dryRun, report); err != nil {
			return nil, err
		}
	}
	if scope.LlmPrices && snapshot.Scope.LlmPrices {
		if err := importLlmPrices(snapshot, mode, dryRun, report); err != nil {
			return nil, err
		}
	}
	if scope.Setting && snapshot.Scope.Setting {
		importSetting(snapshot, mode, dryRun, report)
	}

	return report, nil
}

func importProviders(snapshot *Snapshot, mode string, dryRun bool, report *ImportReport) error {
	stored, err := GetProviders("")
	if err != nil {
		return err
	}

	existing := map[string]*Provider{}
	for _, provider := range stored {
		existing[provider.GetId()] = provider
	}

	named := map[string]bool{}
	for _, incoming := range snapshot.Providers {
		provider := *incoming
		id := provider.GetId()
		named[id] = true

		current, found := existing[id]
		if found && mode == ImportMerge {
			report.record(SectionProviders, id, ChangeSkip, "a provider with this name is already here")
			continue
		}

		if dryRun {
			if found {
				report.record(SectionProviders, id, ChangeUpdate, "")
			} else {
				report.record(SectionProviders, id, ChangeAdd, keylessDetail(&provider))
			}
			continue
		}

		if found {
			// An empty key means the snapshot was taken without secrets, which
			// is "leave the key alone" rather than "clear it" - the same thing
			// the mask means when the form sends it back untouched.
			if provider.ApiKey == "" && current.ApiKey != "" {
				provider.ApiKey = ApiKeyMask
			}
			if provider.Quota != nil && provider.Quota.Token == "" {
				provider.Quota.Token = ApiKeyMask
			}
			if _, err := UpdateProvider(id, &provider); err != nil {
				report.record(SectionProviders, id, ChangeFail, err.Error())
				continue
			}
			report.record(SectionProviders, id, ChangeUpdate, "")
			continue
		}

		detail := keylessDetail(&provider)
		if _, err := AddProvider(&provider); err != nil {
			report.record(SectionProviders, id, ChangeFail, err.Error())
			continue
		}
		report.record(SectionProviders, id, ChangeAdd, detail)
	}

	if mode != ImportReplace {
		return nil
	}

	for _, provider := range stored {
		id := provider.GetId()
		if named[id] {
			continue
		}
		if dryRun {
			report.record(SectionProviders, id, ChangeDelete, "")
			continue
		}
		if _, err := DeleteProvider(provider); err != nil {
			report.record(SectionProviders, id, ChangeFail, err.Error())
			continue
		}
		report.record(SectionProviders, id, ChangeDelete, "")
	}
	return nil
}

// importConnections writes the connections a snapshot carries. Nothing is
// written into an agent's own configuration here: the row keeps the agents it
// names but forgets which Gateway wrote it, so EnsureConnectionsCurrent puts the
// entries in with this machine's port and executable the next time the agents
// are listed. That is also what makes a snapshot restorable onto a machine
// whose Gateway lives somewhere else entirely.
func importConnections(snapshot *Snapshot, mode string, dryRun bool, report *ImportReport) error {
	stored, err := getAllConnections()
	if err != nil {
		return err
	}

	existing := map[string]*Connection{}
	for _, connection := range stored {
		existing[connection.GetId()] = connection
	}

	named := map[string]bool{}
	for _, incoming := range snapshot.Connections {
		connection := *incoming
		id := connection.GetId()
		named[id] = true

		current, found := existing[id]
		if found && mode == ImportMerge {
			report.record(SectionConnections, id, ChangeSkip, "this application is already connected here")
			continue
		}
		if _, known := connector.Get(connection.Name); !known {
			report.record(SectionConnections, id, ChangeSkip, "this Gateway has no connector by that name")
			continue
		}

		change := ChangeAdd
		if found {
			change = ChangeUpdate
		}
		if dryRun {
			report.record(SectionConnections, id, change, credentiallessDetail(&connection))
			continue
		}

		connection.Credentials = keptCredentials(&connection, current)
		// The entries are written again from here, not carried over from
		// wherever this snapshot was taken.
		connection.Endpoint, connection.Executable = "", ""
		detail := credentiallessDetail(&connection)
		if err := saveWithoutRendering(&connection); err != nil {
			report.record(SectionConnections, id, ChangeFail, err.Error())
			continue
		}
		report.record(SectionConnections, id, change, detail)
	}

	if mode != ImportReplace {
		return nil
	}

	for _, connection := range stored {
		id := connection.GetId()
		if named[id] {
			continue
		}
		if dryRun {
			report.record(SectionConnections, id, ChangeDelete, "")
			continue
		}
		if _, err := UninstallConnection(connection.Owner, connection.Name); err != nil {
			report.record(SectionConnections, id, ChangeFail, err.Error())
			continue
		}
		report.record(SectionConnections, id, ChangeDelete, "")
	}
	return nil
}

// keptCredentials layers what the snapshot carries over what is already stored.
// A snapshot taken without secrets carries none, and that means "leave them
// alone" rather than "clear them", the same way an empty provider key does.
func keptCredentials(incoming *Connection, current *Connection) map[string]string {
	credentials := map[string]string{}
	if current != nil {
		for key, value := range current.Credentials {
			credentials[key] = value
		}
	}
	for key, value := range incoming.Credentials {
		if value == "" {
			continue
		}
		credentials[key] = value
	}
	return credentials
}

// credentiallessDetail marks a connection that arrives unusable, which is the
// one thing somebody has to finish by hand after importing a redacted snapshot.
func credentiallessDetail(connection *Connection) string {
	found, ok := connector.Get(connection.Name)
	if !ok {
		return ""
	}
	if _, err := found.Render(connection.Credentials); err != nil {
		return "the snapshot carries no credentials for it"
	}
	return ""
}

// keylessDetail marks a provider that arrives with no key, which is the one
// thing somebody has to finish by hand after importing a redacted snapshot.
func keylessDetail(provider *Provider) string {
	if provider.AuthMode == ProviderAuthSubscription {
		return "the snapshot carries no sign-in for it, so it has to be signed in again"
	}
	if provider.ApiKey == "" && provider.AuthMode != ProviderAuthClient {
		return "the snapshot carries no API key for it"
	}
	return ""
}

func importAgents(snapshot *Snapshot, mode string, dryRun bool, report *ImportReport) error {
	stored, err := GetAgents()
	if err != nil {
		return err
	}

	named := map[string]bool{}
	for _, agent := range snapshot.Agents {
		named[agent.Name] = true
		_, found := stored[agent.Name]
		if found && mode == ImportMerge {
			report.record(SectionAgents, agent.Name, ChangeSkip, "this agent is already routed")
			continue
		}

		action := ChangeAdd
		if found {
			action = ChangeUpdate
		}
		if dryRun {
			report.record(SectionAgents, agent.Name, action, "")
			continue
		}

		// A routing whose provider is not on this machine is refused by
		// SetAgentRouting, which is what the report says rather than the import
		// failing over one agent.
		if err := SetAgentRouting(agent.Name, agent.Provider, agent.Fallbacks, agent.Mode); err != nil {
			report.record(SectionAgents, agent.Name, ChangeFail, err.Error())
			continue
		}
		report.record(SectionAgents, agent.Name, action, "")
	}

	if err := importAgentInstances(snapshot, mode, dryRun, report); err != nil {
		return err
	}

	if err := importAgentPermissions(snapshot, mode, dryRun, report); err != nil {
		return err
	}

	if mode != ImportReplace {
		return nil
	}

	for name := range stored {
		if named[name] {
			continue
		}
		if dryRun {
			report.record(SectionAgents, name, ChangeDelete, "")
			continue
		}
		if err := SetAgentRouting(name, "", nil, ""); err != nil {
			report.record(SectionAgents, name, ChangeFail, err.Error())
			continue
		}
		report.record(SectionAgents, name, ChangeDelete, "")
	}
	return nil
}

// importAgentPermissions restores what each agent is allowed to ask for. A
// permission names nothing outside Gateway, so it is restored as it was taken.
func importAgentPermissions(snapshot *Snapshot, mode string, dryRun bool, report *ImportReport) error {
	stored, err := GetAgentPermissions()
	if err != nil {
		return err
	}

	named := map[string]bool{}
	for _, incoming := range snapshot.AgentPermissions {
		permission := *incoming
		named[permission.Name] = true
		id := permission.Name + " (permissions)"

		_, found := stored[permission.Name]
		if found && mode == ImportMerge {
			report.record(SectionAgents, id, ChangeSkip, "this agent already has permissions")
			continue
		}

		action := ChangeAdd
		if found {
			action = ChangeUpdate
		}
		if dryRun {
			report.record(SectionAgents, id, action, "")
			continue
		}

		if err := UpdateAgentPermission(permission.Name, &permission); err != nil {
			report.record(SectionAgents, id, ChangeFail, err.Error())
			continue
		}
		report.record(SectionAgents, id, action, "")
	}

	if mode != ImportReplace {
		return nil
	}

	// What is left over is put back to unrestricted, which is what an agent
	// nobody has configured is held to.
	for name := range stored {
		if named[name] {
			continue
		}
		id := name + " (permissions)"
		if dryRun {
			report.record(SectionAgents, id, ChangeDelete, "")
			continue
		}
		if err := UpdateAgentPermission(name, DefaultAgentPermission(name)); err != nil {
			report.record(SectionAgents, id, ChangeFail, err.Error())
			continue
		}
		report.record(SectionAgents, id, ChangeDelete, "")
	}
	return nil
}

// importAgentInstances restores the extra copies of an agent. An instance names
// a state directory on the machine it was created on, so one imported from
// elsewhere is a row that only makes sense once that agent is installed here.
func importAgentInstances(snapshot *Snapshot, mode string, dryRun bool, report *ImportReport) error {
	stored, err := GetAgentInstances("")
	if err != nil {
		return err
	}

	existing := map[string]*AgentInstance{}
	for _, instance := range stored {
		existing[instance.Name] = instance
	}

	named := map[string]bool{}
	for _, incoming := range snapshot.AgentInstances {
		instance := *incoming
		named[instance.Name] = true

		if _, found := existing[instance.Name]; found {
			if mode == ImportMerge {
				report.record(SectionAgents, instance.Name, ChangeSkip, "this instance is already here")
				continue
			}
			if dryRun {
				report.record(SectionAgents, instance.Name, ChangeUpdate, "")
				continue
			}
			if err := SetAgentInstanceDisplayName(instance.Name, instance.DisplayName); err != nil {
				report.record(SectionAgents, instance.Name, ChangeFail, err.Error())
				continue
			}
			report.record(SectionAgents, instance.Name, ChangeUpdate, "")
			continue
		}

		if dryRun {
			report.record(SectionAgents, instance.Name, ChangeAdd, "")
			continue
		}
		if err := AddAgentInstance(&instance); err != nil {
			report.record(SectionAgents, instance.Name, ChangeFail, err.Error())
			continue
		}
		report.record(SectionAgents, instance.Name, ChangeAdd, "")
	}

	if mode != ImportReplace {
		return nil
	}

	for _, instance := range stored {
		if named[instance.Name] {
			continue
		}
		if dryRun {
			report.record(SectionAgents, instance.Name, ChangeDelete, "")
			continue
		}
		if err := DeleteAgentInstance(instance.Name); err != nil {
			report.record(SectionAgents, instance.Name, ChangeFail, err.Error())
			continue
		}
		report.record(SectionAgents, instance.Name, ChangeDelete, "")
	}
	return nil
}

func importProbeCases(snapshot *Snapshot, mode string, dryRun bool, report *ImportReport) error {
	stored, err := GetProbeCases()
	if err != nil {
		return err
	}

	existing := map[string]*ProbeCase{}
	for _, probeCase := range stored {
		existing[probeCase.Name] = probeCase
	}

	named := map[string]bool{}
	for _, incoming := range snapshot.ProbeCases {
		probeCase := *incoming
		named[probeCase.Name] = true

		if _, found := existing[probeCase.Name]; found {
			if mode == ImportMerge {
				report.record(SectionProbeCases, probeCase.Name, ChangeSkip, "this case is already here")
				continue
			}
			if dryRun {
				report.record(SectionProbeCases, probeCase.Name, ChangeUpdate, "")
				continue
			}
			if err := UpdateProbeCase(probeCase.Name, &probeCase); err != nil {
				report.record(SectionProbeCases, probeCase.Name, ChangeFail, err.Error())
				continue
			}
			report.record(SectionProbeCases, probeCase.Name, ChangeUpdate, "")
			continue
		}

		if dryRun {
			report.record(SectionProbeCases, probeCase.Name, ChangeAdd, "")
			continue
		}
		if err := AddProbeCase(&probeCase); err != nil {
			report.record(SectionProbeCases, probeCase.Name, ChangeFail, err.Error())
			continue
		}
		report.record(SectionProbeCases, probeCase.Name, ChangeAdd, "")
	}

	if mode != ImportReplace {
		return nil
	}

	for _, probeCase := range stored {
		if named[probeCase.Name] {
			continue
		}
		if dryRun {
			report.record(SectionProbeCases, probeCase.Name, ChangeDelete, "")
			continue
		}
		if err := DeleteProbeCase(probeCase.Name); err != nil {
			report.record(SectionProbeCases, probeCase.Name, ChangeFail, err.Error())
			continue
		}
		report.record(SectionProbeCases, probeCase.Name, ChangeDelete, "")
	}
	return nil
}

func importLlmPrices(snapshot *Snapshot, mode string, dryRun bool, report *ImportReport) error {
	stored, err := GetLlmPriceEntries()
	if err != nil {
		return err
	}

	existing := map[string]*LlmPriceEntry{}
	for _, entry := range stored {
		existing[entry.Model] = entry
	}

	named := map[string]bool{}
	for _, incoming := range snapshot.LlmPrices {
		entry := *incoming
		named[entry.Model] = true

		_, found := existing[entry.Model]
		if found && mode == ImportMerge {
			report.record(SectionLlmPrices, entry.Model, ChangeSkip, "this model is already priced")
			continue
		}

		action := ChangeAdd
		if found {
			action = ChangeUpdate
		}
		if dryRun {
			report.record(SectionLlmPrices, entry.Model, action, "")
			continue
		}
		if err := SetLlmPriceEntry(&entry); err != nil {
			report.record(SectionLlmPrices, entry.Model, ChangeFail, err.Error())
			continue
		}
		report.record(SectionLlmPrices, entry.Model, action, "")
	}

	if mode != ImportReplace {
		return nil
	}

	for _, entry := range stored {
		if named[entry.Model] {
			continue
		}
		if dryRun {
			report.record(SectionLlmPrices, entry.Model, ChangeDelete, "")
			continue
		}
		if err := DeleteLlmPriceEntry(entry.Model); err != nil {
			report.record(SectionLlmPrices, entry.Model, ChangeFail, err.Error())
			continue
		}
		report.record(SectionLlmPrices, entry.Model, ChangeDelete, "")
	}
	return nil
}

// importSetting writes the one settings row. Merge leaves it alone: there is
// always a row, so merging a second one in would mean overwriting the first.
func importSetting(snapshot *Snapshot, mode string, dryRun bool, report *ImportReport) {
	id := BuiltInSettingId
	if mode == ImportMerge {
		report.record(SectionSetting, id, ChangeSkip, "the settings of this Gateway are already set")
		return
	}
	if dryRun {
		report.record(SectionSetting, id, ChangeUpdate, "")
		return
	}

	current, err := GetBuiltInSetting()
	if err != nil {
		report.record(SectionSetting, id, ChangeFail, err.Error())
		return
	}
	if current == nil {
		report.record(SectionSetting, id, ChangeFail, "the built-in setting does not exist")
		return
	}

	setting := *snapshot.Setting
	// A redacted snapshot leaves the secrets behind; the ones already here are
	// kept rather than cleared, which is what makes a keyless snapshot safe to
	// restore onto a working Gateway.
	if setting.ApiKeyEncryptionKey == "" {
		setting.ApiKeyEncryptionKey = current.ApiKeyEncryptionKey
	}
	if setting.ClientSecret == "" {
		setting.ClientSecret = current.ClientSecret
	}
	if setting.RelayToken == "" {
		setting.RelayToken = current.RelayToken
	}

	if _, err := UpdateSetting(id, &setting); err != nil {
		report.record(SectionSetting, id, ChangeFail, err.Error())
		return
	}
	report.record(SectionSetting, id, ChangeUpdate, "")
}
