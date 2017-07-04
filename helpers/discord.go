package helpers

import (
    "errors"
    "github.com/Seklfreak/Robyul2/cache"
    "github.com/bwmarrin/discordgo"
    "time"
    "regexp"
    "math/big"
    "strings"
    "strconv"
    "fmt"
    "github.com/Seklfreak/Robyul2/logger"
)

const (
    DISCORD_EPOCH int64 = 1420070400000
)

var botAdmins = []string{
    "116620585638821891", // Sekl
}
var NukeMods = []string{
    "116620585638821891", // Sekl
    "134298438559858688", // Kakkela
}
var adminRoleNames = []string{"Admin", "Admins", "ADMIN"}
var modRoleNames = []string{"Mod", "Mods", "Mod Trainee", "Moderator", "Moderators", "MOD", "Minimod"}

func IsNukeMod(id string) bool {
    for _, s := range NukeMods {
        if s == id {
            return true
        }
    }

    return false
}

// IsBotAdmin checks if $id is in $botAdmins
func IsBotAdmin(id string) bool {
    for _, s := range botAdmins {
        if s == id {
            return true
        }
    }

    return false
}

func IsAdmin(msg *discordgo.Message) bool {
    channel, e := cache.GetSession().Channel(msg.ChannelID)
    if e != nil {
        return false
    }

    guild, e := cache.GetSession().Guild(channel.GuildID)
    if e != nil {
        return false
    }

    if msg.Author.ID == guild.OwnerID || IsBotAdmin(msg.Author.ID) {
        return true
    }

    guildMember, e := cache.GetSession().GuildMember(guild.ID, msg.Author.ID)
    // Check if role may manage server or a role is in admin role list
    for _, role := range guild.Roles {
        for _, userRole := range guildMember.Roles {
            if userRole == role.ID {
                if role.Permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
                    return true
                }
                for _, adminRoleName := range adminRoleNames {
                    if role.Name == adminRoleName {
                        return true
                    }
                }
            }
        }
    }
    return false
}

func IsMod(msg *discordgo.Message) bool {
    if IsAdmin(msg) == true {
        return true
    } else {
        channel, e := cache.GetSession().Channel(msg.ChannelID)
        if e != nil {
            return false
        }
        guild, e := cache.GetSession().Guild(channel.GuildID)
        if e != nil {
            return false
        }
        guildMember, e := cache.GetSession().GuildMember(guild.ID, msg.Author.ID)
        // check if a role is in mod role list
        for _, role := range guild.Roles {
            for _, userRole := range guildMember.Roles {
                if userRole == role.ID {
                    for _, modRoleName := range modRoleNames {
                        if role.Name == modRoleName {
                            return true
                        }
                    }
                }
            }
        }
    }
    return false
}

// RequireAdmin only calls $cb if the author is an admin or has MANAGE_SERVER permission
func RequireAdmin(msg *discordgo.Message, cb Callback) {
    if !IsAdmin(msg) {
        cache.GetSession().ChannelMessageSend(msg.ChannelID, GetText("admin.no_permission"))
        return
    }

    cb()
}

// RequireAdmin only calls $cb if the author is an admin or has MANAGE_SERVER permission
func RequireMod(msg *discordgo.Message, cb Callback) {
    if !IsMod(msg) {
        cache.GetSession().ChannelMessageSend(msg.ChannelID, GetText("mod.no_permission"))
        return
    }

    cb()
}

// RequireBotAdmin only calls $cb if the author is a bot admin
func RequireBotAdmin(msg *discordgo.Message, cb Callback) {
    if !IsBotAdmin(msg.Author.ID) {
        cache.GetSession().ChannelMessageSend(msg.ChannelID, GetText("botadmin.no_permission"))
        return
    }

    cb()
}

func ConfirmEmbed(channelID string, author *discordgo.User, confirmMessageText string, confirmEmojiID string, abortEmojiID string) bool {
    // send embed asking the user to confirm
    confirmMessage, err := cache.GetSession().ChannelMessageSendEmbedWithMessage(channelID,
        "<@"+author.ID+">",
        &discordgo.MessageEmbed{
            Title:       GetText("bot.embeds.please-confirm-title"),
            Description: confirmMessageText,
        })
    if err != nil {
        cache.GetSession().ChannelMessageSend(channelID, GetTextF("bot.errors.general", err.Error()))
    }

    // delete embed after everything is done
    defer cache.GetSession().ChannelMessageDelete(confirmMessage.ChannelID, confirmMessage.ID)

    // add default reactions to embed
    cache.GetSession().MessageReactionAdd(confirmMessage.ChannelID, confirmMessage.ID, confirmEmojiID)
    cache.GetSession().MessageReactionAdd(confirmMessage.ChannelID, confirmMessage.ID, abortEmojiID)

    // check every second if a reaction has been clicked
    for {
        confirmes, _ := cache.GetSession().MessageReactions(confirmMessage.ChannelID, confirmMessage.ID, confirmEmojiID, 100)
        for _, confirm := range confirmes {
            if confirm.ID == author.ID {
                // user has confirmed the call
                return true
            }
        }
        aborts, _ := cache.GetSession().MessageReactions(confirmMessage.ChannelID, confirmMessage.ID, abortEmojiID, 100)
        for _, abort := range aborts {
            if abort.ID == author.ID {
                // User has aborted the call
                return false
            }
        }

        time.Sleep(1 * time.Second)
    }
}

func GetMuteRole(guildID string) (*discordgo.Role, error) {
    guild, err := cache.GetSession().Guild(guildID)
    Relax(err)
    var muteRole *discordgo.Role
    settings, err := GuildSettingsGet(guildID)
    for _, role := range guild.Roles {
        Relax(err)
        if role.Name == settings.MutedRoleName {
            muteRole = role
        }
    }
    if muteRole == nil {
        muteRole, err = cache.GetSession().GuildRoleCreate(guildID)
        if err != nil {
            return muteRole, err
        }
        muteRole, err = cache.GetSession().GuildRoleEdit(guildID, muteRole.ID, settings.MutedRoleName, muteRole.Color, muteRole.Hoist, 0, muteRole.Mentionable)
        if err != nil {
            return muteRole, err
        }
        for _, channel := range guild.Channels {
            err = cache.GetSession().ChannelPermissionSet(channel.ID, muteRole.ID, "role", 0, discordgo.PermissionSendMessages)
            if err != nil {
                logger.ERROR.L("discord", "Error disabling send messages on mute Role: " + err.Error())
            }
            // TODO: update discordgo
            //err = cache.GetSession().ChannelPermissionSet(channel.ID, muteRole.ID, "role", 0, discordgo.PermissionAddReactions)
            //if err != nil {
            //    logger.ERROR.L("discord", "Error disabling add reactions on mute Role: " + err.Error())
            //}
        }
    }
    return muteRole, nil
}

func GetChannelFromMention(mention string) (*discordgo.Channel, error) {
    var targetChannel *discordgo.Channel
    re := regexp.MustCompile("(<#)?(\\d+)(>)?")
    result := re.FindStringSubmatch(mention)
    if len(result) == 4 {
        targetChannel, err := cache.GetSession().Channel(result[2])
        return targetChannel, err
    } else {
        return targetChannel, errors.New("Channel not found.")
    }
}

func GetUserFromMention(mention string) (*discordgo.User, error) {
    var targetUser *discordgo.User
    re := regexp.MustCompile("(<@)?(\\d+)(>)?")
    result := re.FindStringSubmatch(mention)
    if len(result) == 4 {
        targetUser, err := cache.GetSession().User(result[2])
        return targetUser, err
    } else {
        return targetUser, errors.New("User not found.")
    }
}

func GetDiscordColorFromHex(hex string) int {
    colorInt, ok := new(big.Int).SetString(strings.Replace(hex, "#", "", 1), 16)
    if ok == true {
        return int(colorInt.Int64())
    } else {
        return 0x0FADED
    }
}

func GetTimeFromSnowflake(id string) time.Time {
    iid, err := strconv.ParseInt(id, 10, 64)
    Relax(err)

    return time.Unix(((iid>>22)+DISCORD_EPOCH)/1000, 0).UTC()
}

func GetAllPermissions(guild *discordgo.Guild, member *discordgo.Member) int64 {
    var perms int64 = 0
    for _, x := range guild.Roles {
        if x.Name == "@everyone" {
            perms |= int64(x.Permissions)
        }
    }
    for _, r := range member.Roles {
        for _, x := range guild.Roles {
            if x.ID == r {
                perms |= int64(x.Permissions)
            }
        }
    }
    return perms
}
func Pagify(text string, delimiter string) []string {
    result := make([]string, 0)
    textParts := strings.Split(text, delimiter)
    currentOutputPart := ""
    for _, textPart := range textParts {
        if len(currentOutputPart)+len(textPart)+len(delimiter) <= 1992 {
            if len(currentOutputPart) > 0 || len(result) > 0 {
                currentOutputPart += delimiter + textPart
            } else {
                currentOutputPart += textPart
            }
        } else {
            result = append(result, currentOutputPart)
            currentOutputPart = ""
            if len(textPart) <= 1992 { // @TODO: else: split text somehow
                currentOutputPart = textPart
            }
        }
    }
    if currentOutputPart != "" {
        result = append(result, currentOutputPart)
    }
    return result
}

func GetAvatarUrl(user *discordgo.User) string {
    return GetAvatarUrlWithSize(user, 1024)
}

func GetAvatarUrlWithSize(user *discordgo.User, size uint16) string {
    if user.Avatar == "" {
        return ""
    }

    avatarUrl := "https://cdn.discordapp.com/avatars/%s/%s.%s?size=%d"

    if strings.HasPrefix(user.Avatar, "a_") {
        return fmt.Sprintf(avatarUrl, user.ID, user.Avatar, "gif", size)
    }

    return fmt.Sprintf(avatarUrl, user.ID, user.Avatar, "jpg", size)
}

func CommandExists(name string) bool {
    for _, command := range cache.GetPluginList() {
        if command == strings.ToLower(name) {
            return true
        }
    }
    for _, command := range cache.GetPluginExtendedList() {
        if command == strings.ToLower(name) {
            return true
        }
    }
    for _, command := range cache.GetTriggerPluginList() {
        if command == strings.ToLower(name) {
            return true
        }
    }
    return false
}
