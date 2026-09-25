package main

import (
	"context"
)

// source is where the popup gets its agents and recaps: herdr and Claude, or
// the demo's fictional ones.
type source interface {
	// agents is every agent herdr knows, with its workspaces' names in
	// sidebar order.
	agents() ([]agentInfo, []workspaceInfo, error)
	// session finds the Claude session in a pane.
	session(paneID string) (claudeSession, error)
	// cached is the session's cached recap, if any, and whether it still
	// describes the conversation.
	cached(s claudeSession) (*recap, bool)
	// recap writes a current recap, or with force a new one regardless.
	recap(ctx context.Context, s claudeSession, force bool) (recap, error)
	focus(paneID string) error
}

type liveSource struct{ cfg config }

func (liveSource) agents() ([]agentInfo, []workspaceInfo, error) {
	agents, err := listAgents()
	if err != nil {
		return nil, nil, err
	}
	return agents, listWorkspaces(), nil
}

func (liveSource) session(paneID string) (claudeSession, error) {
	s, err := resolveSession(paneID)
	if err == nil {
		s.Meta = readMeta(s.Transcript)
	}
	return s, err
}

func (liveSource) cached(s claudeSession) (*recap, bool) {
	r := loadRecap(s.ID)
	return r, fresh(r, s)
}

func (l liveSource) recap(ctx context.Context, s claudeSession, force bool) (recap, error) {
	r, _, err := ensureRecap(ctx, l.cfg, s, force)
	return r, err
}

func (liveSource) focus(paneID string) error { return focusAgent(paneID) }
