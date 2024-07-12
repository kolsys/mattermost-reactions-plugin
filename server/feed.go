package main

import (
	"fmt"
	"github.com/mattermost/mattermost/server/public/model"
	"golang.org/x/exp/maps"
)

const REACTION_KEY = "ract:"
const DELETED_REACTION = "__DEL__"


func (p *Plugin) getUsername(userID string) string {
	user, err := p.API.GetUser(userID)
	if err != nil {
		p.API.LogError(
			"Failed to query user",
			"user_id", userID,
			"error", err.Error(),
		)
		return ""
	}
	return user.Username
}


func (p *Plugin) CheckFeedMessage(reaction *model.Reaction) {
	configuration := p.getConfiguration()

	delay := int64(configuration.NotificationDelay)
	rKey := REACTION_KEY + reaction.PostId

	post, err := p.API.GetPost(reaction.PostId)
	if err != nil {
		p.API.LogError(
			"Failed to query post",
			"post_id", reaction.PostId,
			"error", err.Error(),
		)
		return
	}

	userID := post.UserId

	if sentByPlugin, _ := post.GetProp("sent_by_plugin").(bool); sentByPlugin {
		return
	}

	reactions, err := p.API.GetReactions(reaction.PostId)
	if err != nil {
		p.API.LogError(
			"Failed to query reactions",
			"post_id", reaction.PostId,
			"error", err.Error(),
		)
		return
	}

	rPostID := ""
	if brPostID, _ := p.API.KVGet(rKey); brPostID != nil {
		rPostID = string(brPostID)
	}

	// All reactions are deleted
	if len(reactions) == 0 {
		// Delete post with reactions and delay
		if rPostID != "" && rPostID != DELETED_REACTION {
			if err := p.API.DeletePost(rPostID); err != nil {
				p.API.LogError(
					"Failed to remove post",
					"post_id", rPostID,
					"error", err.Error(),
				)
			}
		}
		if err = p.API.KVSetWithExpiry(rKey, []byte(DELETED_REACTION), delay); err != nil {
			p.API.LogError(
				"Failed to set KV",
				"key", rKey,
				"value", DELETED_REACTION,
				"error", err.Error(),
			)
		}
		return
	}

	// Skip youself initialization but allow update
	if userID == reaction.UserId && (rPostID == "" || rPostID == DELETED_REACTION) {
		return
	}

	// Reactions exist, try to send message in direct
  channel, err := p.API.GetDirectChannel(userID, p.botID)
	if err != nil {
		p.API.LogError(
			"Failed to get direct channel",
			"user_id", post.UserId,
			"error", err.Error(),
		)
		return
	}

	rStats := make(map[string]int16)
	uniqueReactors := make(map[string]bool)

	reactionsTXT := ""
	for _, r := range reactions {
		if r.UserId != userID {
			uniqueReactors[r.UserId] = true
		}
		if val, ok := rStats[r.EmojiName]; ok {
			rStats[r.EmojiName] = val + 1
		} else {
			rStats[r.EmojiName] = 1
		}
	}

	reactors := maps.Keys(uniqueReactors)

	firstReactor := "@" + p.getUsername(reactors[0])
	secondReactor := ""
	if len(reactors) == 2 {
		secondReactor = " and @" + p.getUsername(reactors[1])
	} else if len(reactors) > 1 {
		secondReactor = " and several others"
	}

	for emoji, val := range(rStats) {
		reactionsTXT = fmt.Sprintf("%s:%s: %d ", reactionsTXT, emoji, val)
	}
	postURL := fmt.Sprintf("%s/_redirect/pl/%s", *p.API.GetConfig().ServiceSettings.SiteURL, reaction.PostId)

	msg := fmt.Sprintf("%s%s reacted to your message\n%s\n%s", firstReactor, secondReactor, reactionsTXT, postURL)


	// No pushed yet, will create new post
	if rPostID == "" || rPostID == DELETED_REACTION {
		post, err := p.API.CreatePost(&model.Post{
			ChannelId: channel.Id,
			UserId: p.botID,
			Message: msg,
			Props: map[string]any{
				"sent_by_plugin": true,
			},
		})
		if err != nil {
			p.API.LogError(
				"Failed to create post",
				"user_id", userID,
				"error", err.Error(),
			)
			return
		}
		rPostID = post.Id
	// Notification already exist, just update it
	} else {
		post, err := p.API.UpdatePost(&model.Post{
			Id: rPostID,
			ChannelId: channel.Id,
			Message: msg,
			Props: map[string]any{
				"sent_by_plugin": true,
			},
		})
		if err != nil {
			p.API.LogError(
				"Failed to update post",
				"user_id", userID,
				"post_id", rPostID,
				"error", err.Error(),
			)
			return
		}

		rPostID = post.Id
	}
	// Mark post for delay notifications
	if err = p.API.KVSetWithExpiry(rKey, []byte(rPostID), delay); err != nil {
		p.API.LogError(
			"Failed to set KV",
			"key", rKey,
			"value", DELETED_REACTION,
			"error", err.Error(),
		)
	}
}
