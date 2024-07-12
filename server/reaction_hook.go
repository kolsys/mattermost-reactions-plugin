package main

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) ReactionHasBeenAdded(c *plugin.Context, reaction *model.Reaction) {
	p.CheckFeedMessage(reaction)
}

func (p *Plugin) ReactionHasBeenRemoved(c *plugin.Context, reaction *model.Reaction) {
	p.CheckFeedMessage(reaction)
}
