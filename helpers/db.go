package helpers

import (
	"sync"
	"time"

	"github.com/Seklfreak/Robyul2/cache"
	"github.com/Seklfreak/Robyul2/models"
	"github.com/getsentry/raven-go"
	"github.com/globalsign/mgo/bson"
	rethink "github.com/gorethink/gorethink"
)

var (
	dbSession *rethink.Session

	guildSettingsCache map[string]models.Config
	cacheMutex         sync.RWMutex
)

// ConnectDB connects to rethink and stores the session
func ConnectDB(url string, db string) {
	log := cache.GetLogger()
	log.WithField("module", "db").Info("Connecting to " + url)

	rethink.Log = cache.GetLogger()
	rethink.SetTags("rethink", "json")
	rethink.SetVerbose(true)

	session, err := rethink.Connect(rethink.ConnectOpts{
		Address:  url,
		Database: db,
	})

	if err != nil {
		log.WithField("module", "db").Error(err.Error())
		panic(err)
	}

	dbSession = session

	cacheMutex.Lock()
	guildSettingsCache = make(map[string]models.Config)
	cacheMutex.Unlock()

	log.WithField("module", "db").Info("Connected!")
}

// GetDB is a simple getter for the rethink session.
// Might receive some singleton-like lazy-creation later
func GetDB() *rethink.Session {
	return dbSession
}

// GuildSettingsSet writes all $config into the db
func GuildSettingsSet(guild string, config models.Config) error {
	// Check if an config object exists
	var settings models.Config

	err := MdbOne(
		MdbCollection(models.GuildConfigTable).Find(bson.M{"guildid": guild}),
		&settings,
	)

	if IsMdbNotFound(err) {
		_, err = MDbInsert(
			models.GuildConfigTable,
			config,
		)
	} else if err != nil {
		return err
	} else {
		err = MDbUpdate(models.GuildConfigTable, config.ID, config)
	}
	if err != nil {
		return err
	}

	// Update cache
	cacheMutex.Lock()
	guildSettingsCache[guild] = config
	cacheMutex.Unlock()

	return err
}

// GuildSettingsGet returns all config values for the guild or a default object
func GuildSettingsGet(guild string) (models.Config, error) {
	var settings models.Config
	var err error

	err = MdbOne(
		MdbCollection(models.GuildConfigTable).Find(bson.M{"guildid": guild}),
		&settings,
	)

	if IsMdbNotFound(err) {
		settings = models.Config{}.Default(guild)
		return settings, nil
	}

	return settings, err
}

func GuildSettingsGetCached(id string) models.Config {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	settings := guildSettingsCache[id]
	return settings
}

// GetPrefixForServer gets the prefix for $guild
func GetPrefixForServer(guildID string) string {
	return GuildSettingsGetCached(guildID).Prefix
}

// SetPrefixForServer sets the prefix for $guild to $prefix
func SetPrefixForServer(guild string, prefix string) error {
	settings := GuildSettingsGetCached(guild)

	settings.Prefix = prefix

	return GuildSettingsSet(guild, settings)
}

func GuildSettingsUpdater() {
	for {
		for _, guild := range cache.GetSession().State.Guilds {
			settings, e := GuildSettingsGet(guild.ID)
			if e != nil {
				raven.CaptureError(e, map[string]string{})
				continue
			}

			cacheMutex.Lock()
			guildSettingsCache[guild.ID] = settings
			cacheMutex.Unlock()
		}

		time.Sleep(15 * time.Second)
	}
}
