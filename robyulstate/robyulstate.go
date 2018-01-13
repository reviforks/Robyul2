package robyulstate

import (
	"sync"

	"fmt"

	"github.com/Seklfreak/Robyul2/helpers"
	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	"github.com/getsentry/raven-go"
	"github.com/olivere/elastic"
)

type Robyulstate struct {
	sync.RWMutex

	emojiMap map[string][]*discordgo.Emoji

	Logger func(msgL, caller int, format string, a ...interface{})
}

func NewState() *Robyulstate {
	return &Robyulstate{
		emojiMap: make(map[string][]*discordgo.Emoji),
	}
}

func (s *Robyulstate) OnInterface(_ *discordgo.Session, i interface{}) {
	defer func() {
		err := recover()
		if err != nil {
			s.Logger(discordgo.LogError, 0, fmt.Sprintf("Recover: %s", spew.Sdump(err)))

			if errE, ok := err.(*elastic.Error); ok {
				raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{
					"Type":     errE.Details.Type,
					"Reason":   errE.Details.Reason,
					"Index":    errE.Details.Index,
					"CausedBy": spew.Sdump(errE.Details.CausedBy),
				})
			} else {
				raven.CaptureError(fmt.Errorf(spew.Sdump(err)), map[string]string{})
			}
		}
	}()

	if s == nil {
		s.Logger(discordgo.LogError, 0, discordgo.ErrNilState.Error())
		return
	}

	var err error

	//fmt.Println("received event:", reflect.TypeOf(i))

	switch t := i.(type) {
	case *discordgo.GuildCreate:
		err = s.GuildAdd(t.Guild)
	case *discordgo.GuildUpdate:
		err = s.GuildAdd(t.Guild)
	case *discordgo.GuildDelete:
		err = s.GuildRemove(t.Guild)
	case *discordgo.GuildEmojisUpdate:
		err = s.EmojisUpdate(t.GuildID, t.Emojis)
		/*
			case *GuildMemberAdd:
				if s.TrackMembers {
					err = s.MemberAdd(t.Member)
				}
			case *GuildMemberUpdate:
				if s.TrackMembers {
					err = s.MemberAdd(t.Member)
				}
			case *GuildMemberRemove:
				if s.TrackMembers {
					err = s.MemberRemove(t.Member)
				}
			case *GuildMembersChunk:
				if s.TrackMembers {
					for i := range t.Members {
						t.Members[i].GuildID = t.GuildID
						err = s.MemberAdd(t.Members[i])
					}
				}
			case *GuildRoleCreate:
				if s.TrackRoles {
					err = s.RoleAdd(t.GuildID, t.Role)
				}
			case *GuildRoleUpdate:
				if s.TrackRoles {
					err = s.RoleAdd(t.GuildID, t.Role)
				}
			case *GuildRoleDelete:
				if s.TrackRoles {
					err = s.RoleRemove(t.GuildID, t.RoleID)
				}
			case *ChannelCreate:
				if s.TrackChannels {
					err = s.ChannelAdd(t.Channel)
				}
			case *ChannelUpdate:
				if s.TrackChannels {
					err = s.ChannelAdd(t.Channel)
				}
			case *ChannelDelete:
				if s.TrackChannels {
					err = s.ChannelRemove(t.Channel)
				}
			case *MessageCreate:
				if s.MaxMessageCount != 0 {
					err = s.MessageAdd(t.Message)
				}
			case *MessageUpdate:
				if s.MaxMessageCount != 0 {
					err = s.MessageAdd(t.Message)
				}
			case *MessageDelete:
				if s.MaxMessageCount != 0 {
					err = s.MessageRemove(t.Message)
				}
			case *MessageDeleteBulk:
				if s.MaxMessageCount != 0 {
					for _, mID := range t.Messages {
						s.messageRemoveByID(t.ChannelID, mID)
					}
				}
			case *VoiceStateUpdate:
				if s.TrackVoice {
					err = s.voiceStateUpdate(t)
				}
			case *PresenceUpdate:
				if s.TrackPresences {
					s.PresenceAdd(t.GuildID, &t.Presence)
				}
				if s.TrackMembers {
					if t.Status == StatusOffline {
						return
					}

					var m *Member
					m, err = s.Member(t.GuildID, t.User.ID)

					if err != nil {
						// Member not found; this is a user coming online
						m = &Member{
							GuildID: t.GuildID,
							Nick:    t.Nick,
							User:    t.User,
							Roles:   t.Roles,
						}

					} else {

						if t.Nick != "" {
							m.Nick = t.Nick
						}

						if t.User.Username != "" {
							m.User.Username = t.User.Username
						}

						// PresenceUpdates always contain a list of roles, so there's no need to check for an empty list here
						m.Roles = t.Roles

					}

					err = s.MemberAdd(m)
				}
		*/

	}

	if err != nil {
		s.Logger(discordgo.LogError, 0, err.Error())
	}

	return
}

func (s *Robyulstate) GuildAdd(guild *discordgo.Guild) error {
	if s == nil {
		return discordgo.ErrNilState
	}

	s.Lock()
	defer s.Unlock()

	s.emojiMap[guild.ID] = make([]*discordgo.Emoji, len(guild.Emojis))
	copy(s.emojiMap[guild.ID], guild.Emojis)

	return nil
}

func (s *Robyulstate) GuildRemove(guild *discordgo.Guild) error {
	if s == nil {
		return discordgo.ErrNilState
	}

	s.Lock()
	defer s.Unlock()

	s.emojiMap[guild.ID] = nil

	return nil
}

func (s *Robyulstate) EmojisUpdate(guildID string, emojis []*discordgo.Emoji) error {
	if s == nil {
		return discordgo.ErrNilState
	}

	s.Lock()
	defer s.Unlock()

	if _, ok := s.emojiMap[guildID]; !ok {
		s.emojiMap[guildID] = emojis
	}

	// remove guild emoji not in emojis
	for i, oldEmoji := range s.emojiMap[guildID] {
		emojiRemoved := true
		for _, newEmoji := range emojis {
			if newEmoji.ID == oldEmoji.ID {
				emojiRemoved = false
			}
		}
		if emojiRemoved {
			s.emojiMap[guildID] = append(s.emojiMap[guildID][:i], s.emojiMap[guildID][i+1:]...)
			// emoji got removed
			//fmt.Println("removed", oldEmoji.Name)
			helpers.OnEmojiDelete(guildID, oldEmoji)
		}
	}

	// update guild emoji
	for j, oldEmoji := range s.emojiMap[guildID] {
		for i, newEmoji := range emojis {
			if oldEmoji.ID == newEmoji.ID {
				if oldEmoji.Name != newEmoji.Name ||
					oldEmoji.Animated != newEmoji.Animated ||
					oldEmoji.RequireColons != newEmoji.RequireColons ||
					oldEmoji.Managed != newEmoji.Managed {
					// emoji got updated
					//fmt.Println("update", oldEmoji.Name, "to", newEmoji.Name)
					helpers.OnEmojiUpdate(guildID, oldEmoji, newEmoji)
				}
				emojis = append(emojis[:i], emojis[i+1:]...)
				s.emojiMap[guildID][j] = newEmoji
			}
		}
	}

	// add guild emoji
	for _, newEmoji := range emojis {
		s.emojiMap[guildID] = append(s.emojiMap[guildID], newEmoji)
		// emoji got added
		//fmt.Println("added", newEmoji.Name)
		helpers.OnEmojiCreate(guildID, newEmoji)
	}

	return nil
}
