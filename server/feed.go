package main

import (
	"fmt"
	"strconv"
	"github.com/mattermost/mattermost/server/public/model"
	"golang.org/x/exp/maps"
)

const REACTION_KEY = "ract:"
const REACTION_OFF_KEY = "off:"
const REACTIONS_DELETED_MSG = "(Reactions deleted)"

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

func (p *Plugin) buildReactionsMessage(reactions []*model.Reaction, userID string, offset int64, needOffset bool) string {
	if len(reactions) == 0 {
		return REACTIONS_DELETED_MSG
	}

	rStats := make(map[string]int16)
	uniqueReactors := make(map[string]bool)
	for _, r := range reactions {
		if !needOffset || r.CreateAt >= offset {
			if r.UserId != userID {
				uniqueReactors[r.UserId] = true
			}
			if val, ok := rStats[r.EmojiName]; ok {
				rStats[r.EmojiName] = val + 1
			} else {
				rStats[r.EmojiName] = 1
			}
		}
	}

	reactors := maps.Keys(uniqueReactors)

	firstReactor := "Someone"
	secondReactor := ""
	if len(reactors) > 0 {
		firstReactor = "@" + p.getUsername(reactors[0])
	} else {
		return REACTIONS_DELETED_MSG
	}
	if len(reactors) == 2 {
		secondReactor = " and @" + p.getUsername(reactors[1])
	} else if len(reactors) > 1 {
		secondReactor = " and several others"
	}

	emojis := ""

	for emoji, val := range(rStats) {
		emojis = fmt.Sprintf("%s:%s: %d ", emojis, emoji, val)
	}
	return fmt.Sprintf("%s%s reacted to your message\n%s", firstReactor, secondReactor, emojis)
}

func (p *Plugin) CheckFeedMessage(reaction *model.Reaction) {
	configuration := p.getConfiguration()

	delay := int64(configuration.NotificationDelay)
	needOffset := bool(configuration.ShowOnlyNew)
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

	// Do not process reactions posts
	if sentByPlugin, _ := post.GetProp("sent_by_plugin").(bool); sentByPlugin {
		return
	}
	// Skip if user is tunrned off notifications by command
	if isTurnedOff, _ := p.API.KVGet(REACTION_OFF_KEY + userID); isTurnedOff != nil {
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

	// All reactions deleted, nothing to do
	if len(reactions) == 0 && rPostID == "" {
		return
	}

	// Skip youself initialization but allow update
	if userID == reaction.UserId && rPostID == "" {
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

	var offset int64 = 0

	if rPostID != "" {
		rPost, err := p.API.GetPost(rPostID)
		if err != nil {
			p.API.LogError(
				"Could not find previous post",
				"post_id", rPostID,
				"error", err.Error(),
			)
			rPostID = ""
		} else {
			strOffset, _ := rPost.GetProp("reaction_offset").(string)
			if pOffset, err := strconv.Atoi(strOffset); err == nil {
				offset = int64(pOffset)
			}
		}
	}

	if offset == 0 {
		// If post offset not found and reactions more than one then get current reaction as offset
		if len(reactions) > 1 {
			offset = reaction.CreateAt
		} else if len(reactions) > 0 {
			offset = reactions[0].CreateAt
		}
	}

	msg := p.buildReactionsMessage(reactions, userID, offset, needOffset)

	postURL := fmt.Sprintf("%s/_redirect/pl/%s", *p.API.GetConfig().ServiceSettings.SiteURL, reaction.PostId)
	msg = fmt.Sprintf("%s\n%s", msg, postURL)

	// No pushed yet, will create new post
	if rPostID == "" {
		post, err := p.API.CreatePost(&model.Post{
			ChannelId: channel.Id,
			UserId: p.botID,
			Message: msg,
			Props: map[string]any{
				"sent_by_plugin": true,
				"reaction_offset": fmt.Sprintf("%d", offset),
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
				"reaction_offset": fmt.Sprintf("%d", offset),
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
			"value", rPostID,
			"error", err.Error(),
		)
	}
}
