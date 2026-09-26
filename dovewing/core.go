package dovewing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/PlexiOSS/Keel/dovewing/dovetypes"
	"github.com/PlexiOSS/Keel/hotcache"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

type BaseState struct {
	Logger            *zap.Logger
	Context           context.Context
	Pool              *pgxpool.Pool
	PlatformUserCache hotcache.HotCache[dovetypes.PlatformUser]
	Middlewares       []func(p Platform, u *dovetypes.PlatformUser) (*dovetypes.PlatformUser, error)
	UserExpiryTime    time.Duration
	StaleRetryTime    time.Duration
}

const defaultStaleRetryTime = time.Minute

var refreshesInFlight sync.Map

type Platform interface {
	Init() error
	Initted() bool
	GetState() *BaseState
	PlatformName() string
	ValidateId(id string) (string, error)
	PlatformSpecificCache(ctx context.Context, id string) (*dovetypes.PlatformUser, error)
	GetUser(ctx context.Context, id string) (*dovetypes.PlatformUser, error)
}

func InitPlatform(platform Platform) error {
	state := platform.GetState()

	var tableName = TableName(platform)

	_, err := state.Pool.Exec(state.Context, `
		CREATE TABLE IF NOT EXISTS `+tableName+` (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			display_name TEXT NOT NULL,
			avatar TEXT NOT NULL,
			bot BOOLEAN NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)

	if err != nil {
		return err
	}

	return platform.Init()
}

func TableName(platform Platform) string {
	return "internal_user_cache__" + platform.PlatformName()
}

func GetUser(ctx context.Context, id string, platform Platform) (*dovetypes.PlatformUser, error) {
	state := platform.GetState()

	if err := ensureInit(platform); err != nil {
		return nil, err
	}

	var platformName = platform.PlatformName()
	var tableName = TableName(platform)

	uCached, err := platform.PlatformSpecificCache(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("platformSpecificCache failed: %s", err)
	}

	if uCached != nil {
		return persistFresh(platform, uCached)
	}

	user, err := state.PlatformUserCache.Get(ctx, platformName+":"+id)

	if err != nil && err != hotcache.ErrHotCacheDataNotFound {
		return nil, fmt.Errorf("failed to get user from redis cache: %s", err)
	}

	if err == nil {
		user.ExtraData = map[string]any{
			"cache": "redis",
		}

		for i, middleware := range state.Middlewares {
			user, err = middleware(platform, user)

			if err != nil {
				return nil, fmt.Errorf("middleware %d failed: %s", i, err)
			}
		}

		return user, nil
	}

	var (
		username    string
		displayName string
		avatar      string
		bot         bool
		lastUpdated time.Time
	)

	err = state.Pool.QueryRow(ctx, "SELECT username, display_name, avatar, bot, last_updated FROM "+tableName+" WHERE id = $1", id).Scan(&username, &displayName, &avatar, &bot, &lastUpdated)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		state.Logger.Warn("Failed to check internal user cache.", zap.Error(err), zap.String("id", id), zap.String("platform", platformName), zap.String("tableName", tableName))
	}

	if err == nil {
		row := &dovetypes.PlatformUser{
			ID:          id,
			Username:    username,
			Avatar:      avatar,
			DisplayName: displayName,
			Bot:         bot,
			Status:      dovetypes.PlatformStatusOffline,
			ExtraData: map[string]any{
				"cache": "pg",
			},
		}

		age := time.Since(lastUpdated)
		hotCacheTTL := state.UserExpiryTime - age

		if hotCacheTTL <= 0 {
			refreshInBackground(platform, id)

			hotCacheTTL = state.StaleRetryTime

			if hotCacheTTL <= 0 {
				hotCacheTTL = defaultStaleRetryTime
			}
		}

		row, err = applyMiddlewares(platform, row)

		if err != nil {
			return nil, err
		}

		if err := state.PlatformUserCache.Set(state.Context, platformName+":"+id, row, hotCacheTTL); err != nil {
			state.Logger.Warn("Failed to set user in hot cache", zap.Error(err), zap.String("id", id), zap.String("platform", platformName))
		}

		return row, nil
	}

	user, err = platform.GetUser(ctx, id)

	if err != nil {
		return nil, errors.New("failed to get user from platform: " + err.Error())
	}

	return persistFresh(platform, user)
}

func ensureInit(platform Platform) error {
	if platform.Initted() {
		return nil
	}

	if err := InitPlatform(platform); err != nil {
		return errors.New("failed to init platform: " + err.Error())
	}

	if !platform.Initted() {
		return errors.New("platform init() did not set initted() to true")
	}

	return nil
}

func applyMiddlewares(platform Platform, u *dovetypes.PlatformUser) (*dovetypes.PlatformUser, error) {
	if u.DisplayName == "" {
		u.DisplayName = u.Username
	}

	var err error

	for i, middleware := range platform.GetState().Middlewares {
		u, err = middleware(platform, u)

		if err != nil {
			return nil, fmt.Errorf("middleware %d failed: %s", i, err)
		}
	}

	return u, nil
}

func persistFresh(platform Platform, u *dovetypes.PlatformUser) (*dovetypes.PlatformUser, error) {
	if u == nil {
		return nil, errors.New("user not found")
	}

	state := platform.GetState()
	platformName := platform.PlatformName()

	u, err := applyMiddlewares(platform, u)

	if err != nil {
		return nil, err
	}

	_, err = state.Pool.Exec(state.Context, "INSERT INTO "+TableName(platform)+" (id, username, display_name, avatar, bot) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO UPDATE SET username = $2, display_name = $3, avatar = $4, bot = $5, last_updated = NOW()", u.ID, u.Username, u.DisplayName, u.Avatar, u.Bot)

	if err != nil {
		return nil, fmt.Errorf("failed to update internal user cache: %s", err)
	}

	if err := state.PlatformUserCache.Set(state.Context, platformName+":"+u.ID, u, state.UserExpiryTime); err != nil {
		state.Logger.Warn("Failed to set user in hot cache", zap.Error(err), zap.String("id", u.ID), zap.String("platform", platformName))
	}

	return u, nil
}

func RefreshUser(ctx context.Context, id string, platform Platform) (*dovetypes.PlatformUser, error) {
	if err := ensureInit(platform); err != nil {
		return nil, err
	}

	key := platform.PlatformName() + ":" + id

	if _, alreadyRunning := refreshesInFlight.LoadOrStore(key, struct{}{}); alreadyRunning {
		return nil, nil
	}

	defer refreshesInFlight.Delete(key)

	user, err := platform.PlatformSpecificCache(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("platformSpecificCache failed: %s", err)
	}

	if user == nil {
		user, err = platform.GetUser(ctx, id)

		if err != nil {
			return nil, errors.New("failed to get user from platform: " + err.Error())
		}
	}

	return persistFresh(platform, user)
}

func refreshInBackground(platform Platform, id string) {
	state := platform.GetState()
	platformName := platform.PlatformName()
	key := platformName + ":" + id

	if _, alreadyRunning := refreshesInFlight.LoadOrStore(key, struct{}{}); alreadyRunning {
		return
	}

	go func() {
		defer refreshesInFlight.Delete(key)

		ctx, cancel := context.WithTimeout(state.Context, 30*time.Second)
		defer cancel()

		state.Logger.Info("Updating expired user cache", zap.String("id", id), zap.String("platform", platformName))

		user, err := platform.GetUser(ctx, id)

		if err != nil {
			state.Logger.Error("Failed to update expired user cache; will retry on a later lookup", zap.Error(err), zap.String("id", id), zap.String("platform", platformName))
			return
		}

		if _, err := persistFresh(platform, &dovetypes.PlatformUser{
			ID:          id,
			Username:    user.Username,
			Avatar:      user.Avatar,
			DisplayName: user.DisplayName,
			Bot:         user.Bot,
			Status:      user.Status,
		}); err != nil {
			state.Logger.Error("Failed to persist refreshed user", zap.Error(err), zap.String("id", id), zap.String("platform", platformName))
		}
	}()
}

type ClearFrom string

const (
	ClearFromInternalUserCache ClearFrom = "iuc"
	ClearFromRedis             ClearFrom = "redis"
)

type ClearUserInfo struct {
	ClearedFrom []ClearFrom
	IsBot       bool
}

type ClearUserReq struct {
	ClearFrom []ClearFrom
}

func ClearUser(ctx context.Context, id string, platform Platform, req ClearUserReq) (*ClearUserInfo, error) {
	state := platform.GetState()

	if err := ensureInit(platform); err != nil {
		return nil, err
	}

	var platformName = platform.PlatformName()
	var tableName = TableName(platform)

	var clearedFrom []ClearFrom
	var isBot bool

	err := state.Pool.QueryRow(ctx, "SELECT bot FROM "+tableName+" WHERE id = $1", id).Scan(&isBot)

	if err != nil {
		return nil, err
	}

	if len(req.ClearFrom) == 0 || slices.Contains(req.ClearFrom, ClearFromInternalUserCache) {
		var count int64

		err := state.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+tableName+" WHERE id = $1", id).Scan(&count)

		if err != nil {
			return nil, err
		}

		if count > 0 {
			_, err = state.Pool.Exec(ctx, "DELETE FROM "+tableName+" WHERE id = $1", id)

			if err != nil {
				return nil, err
			}

			clearedFrom = append(clearedFrom, ClearFromInternalUserCache)
		}
	}

	if len(req.ClearFrom) == 0 || slices.Contains(req.ClearFrom, ClearFromRedis) {
		err := state.PlatformUserCache.Delete(ctx, platformName+":"+id)

		if err != nil {
			return nil, err
		}

		clearedFrom = append(clearedFrom, ClearFromRedis)
	}

	return &ClearUserInfo{
		ClearedFrom: clearedFrom,
		IsBot:       isBot,
	}, nil
}
