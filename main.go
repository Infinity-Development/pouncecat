package main

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"pouncecat/column"
	"pouncecat/helpers"
	"pouncecat/source/mongo"
	"pouncecat/table"
	"pouncecat/transform"
	"pouncecat/ui"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/exp/slices"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	err := godotenv.Load()

	if err != nil {
		panic(err)
	}
}

func main() {

	sess, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))

	if err != nil {
		panic(err)
	}

	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

	err = sess.Open()

	if err != nil {
		panic(err)
	}

	source := mongo.MongoSource{
		ConnectionURL:  os.Getenv("MONGO"),
		DatabaseName:   "infinity",
		IgnoreEntities: []string{"sessions"},
	}

	err = source.Connect()

	if err != nil {
		panic(err)
	}

	pool, err := pgxpool.Connect(context.Background(), "postgresql://127.0.0.1:5432/infinity?user=root&password=iblpublic")

	if err != nil {
		panic(err)
	}

	table.PrepareTables(pool)

	table.Table{
		SrcName:           "users",
		DstName:           "users",
		IgnoreUniqueError: true,
		Columns: column.Columns(
			column.NewText(
				column.Source("userID"),
				column.Dest("user_id"),
				column.Default("SKIP"),
				func(records map[string]any, p any) any {
					if p == nil {
						return p
					}

					userId := p.(string)

					return strings.TrimSpace(userId)
				},
			).SetUnique(true),
			column.NewText(
				column.Source("username"),
				column.Dest("username"),
				nil,
				func(records map[string]any, p any) any {
					userId := records["userID"].(string)

					username, ok := p.(string)

					if ok && username != "" {
						return username
					}

					// Call http://localhost:8080/_duser/ID
					resp, err := http.Get("http://localhost:8080/_duser/" + userId)

					if err != nil {
						fmt.Println("User fetch error:", err)
						return "Unknown User"
					}

					if resp.StatusCode != 200 {
						fmt.Println("User fetch error:", resp.StatusCode)
						return "Unknown User"
					}

					// Read the response body
					body, err := io.ReadAll(resp.Body)

					if err != nil {
						fmt.Println("User fetch error:", err)
						return "Unknown User"
					}

					var data struct {
						Username string `json:"username"`
					}

					// Unmarshal the response body
					err = json.Unmarshal(body, &data)

					if err != nil {
						fmt.Println("User fetch error:", err)
						return "Unknown User"
					}

					return data.Username
				},
			),
			column.NewBool(
				column.Source("staff_onboarded"),
				column.Dest("staff_onboarded"),
				column.Default(false),
			),
			column.NewText(
				column.Source("staff_onboard_state"),
				column.Dest("staff_onboard_state"),
				column.Default("pending"),
			),
			column.NewTimestamp(
				column.Source("staff_onboard_last_start_time"),
				column.Dest("staff_onboard_last_start_time"),
				"",
			).SetNullable(true),
			column.NewTimestamp(
				column.Source("staff_onboard_macro_time"),
				column.Dest("staff_onboard_macro_time"),
				"",
			).SetNullable(true),
			column.NewText(
				column.Source("staff_onboard_session_code"),
				column.Dest("staff_onboard_session_code"),
				nil,
			).SetNullable(true),
			column.NewBool(
				column.Source("staff"),
				column.Dest("staff"),
				column.Default(false),
			),
			column.NewBool(
				column.Source("admin"),
				column.Dest("admin"),
				column.Default(false),
			),
			column.NewBool(
				column.Source("hadmin"),
				column.Dest("hadmin"),
				column.Default(false),
			),
			column.NewJSONB(
				column.Source("extra_links"),
				column.Dest("extra_links"),
				func(record map[string]any, col any) any {
					parsedLinks := []link{}
					for _, name := range []string{"website", "github"} {
						linkC, ok := record[name]

						if !ok {
							continue
						}

						linkStr, ok := linkC.(string)

						if !ok {
							continue
						}

						// Title-case name
						name = cases.Title(language.AmericanEnglish).String(name)

						parsedLink := parseLink(name, linkStr)

						if parsedLink != "" {
							parsedLinks = append(parsedLinks, link{
								Name:  name,
								Value: parsedLink,
							})
						}
					}

					return parsedLinks
				},
			),
			column.NewText(
				column.Source("apiToken"),
				column.Dest("api_token"),
				nil,
				func(record map[string]any, col any) any {
					if col == nil {
						return helpers.RandString(128)
					}

					return col
				},
			),
			column.NewText(
				column.Source("about"),
				column.Dest("about"),
				"I am a very mysterious person",
			),
			column.NewBool(
				column.Source("vote_banned"),
				column.Dest("vote_banned"),
				column.Default(false),
			),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName: "apps",
		DstName: "apps",
		Columns: column.Columns(
			column.NewText(
				column.Source("appID"),
				column.Dest("app_id"),
				column.NoDefault,
			),
			column.NewText(
				column.Source("userID"),
				column.Dest("user_id"),
				column.NoDefault,
			).SetForeignKey([2]string{"users", "user_id"}),
			column.NewText(
				column.Source("position"),
				column.Dest("position"),
				column.NoDefault,
			),
			column.NewTimestamp(
				column.Source("created_at"),
				column.Dest("created_at"),
				"NOW()", // Default to now
				transform.ToTimestamp,
			),
			column.NewJSONB(
				column.Source("answers"),
				column.Dest("answers"),
			),
			column.NewJSONB(
				column.Source("interviewAnswers"),
				column.Dest("interview_answers"),
			),
			column.NewText(
				column.Source("state"),
				column.Dest("state"),
				"pending",
			),
			column.NewBigInt(
				column.Source("likes"),
				column.Dest("likes"),
				column.ArrayJSONDefault,
			).SetArray(true),
			column.NewBigInt(
				column.Source("dislikes"),
				column.Dest("dislikes"),
				column.ArrayJSONDefault,
			).SetArray(true),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName:   "bots",
		DstName:   "bots",
		IndexCols: []string{"bot_id", "staff_bot", "cross_add", "api_token", "lower(vanity)"},
		Columns: column.Columns(
			column.NewText(
				column.Source("botID"),
				column.Dest("bot_id"),
				column.Default("SKIP"),
				func(records map[string]any, p any) any {
					if p == nil {
						return p
					}

					userId := p.(string)

					return strings.TrimSpace(userId)
				},
			).SetUnique(true),
			column.NewText(
				column.Source("clientID"),
				column.Dest("client_id"),
				column.Default(column.NoDefault),
				func(record map[string]any, col any) any {
					botId := record["botID"].(string)

					if col == nil {
						ui.NotifyMsg("info", "No client ID for bot "+botId+", finding one")
						// Call http://localhost:8080/_duser/ID
						resp, err := http.Get("http://localhost:8080/_duser/" + botId)

						if err != nil {
							fmt.Println("User fetch error:", err)
							return "SKIP"
						}

						if resp.StatusCode != 200 {
							fmt.Println("User fetch error:", resp.StatusCode)
							return "SKIP"
						}

						_, rerr := sess.Request("GET", "https://discord.com/api/v10/applications/"+botId+"/rpc", nil)

						if rerr == nil {
							source.Conn.Database("infinity").Collection("bots").UpdateOne(context.Background(), bson.M{
								"botID": botId,
							}, bson.M{
								"$set": bson.M{
									"clientID": botId,
								},
							})
						}

						for rerr != nil {
							clientId := helpers.PromptServerChannel("What is the client ID for " + botId + "?")

							if clientId == "DEL" {
								source.Conn.Database("infinity").Collection("bots").DeleteOne(context.Background(), bson.M{"botID": botId})
								return "SKIP"
							}

							_, rerr = sess.Request("GET", "https://discord.com/api/v10/applications/"+clientId+"/rpc", nil)

							if rerr != nil {
								fmt.Println("Client ID fetch error:", rerr)
								continue
							}

							source.Conn.Database("infinity").Collection("bots").UpdateOne(context.Background(), bson.M{
								"botID": botId,
							}, bson.M{
								"$set": bson.M{
									"clientID": clientId,
								},
							})

							return clientId
						}

						return botId
					}

					return col
				},
			),
			column.NewText(
				column.Source("botName"),
				column.Dest("queue_name"),
				column.NoDefault,
			),
			column.NewText(
				column.Source("tags"),
				column.Dest("tags"),
				column.ArrayJSONDefault,
				transform.ToList,
			).SetArray(true),
			column.NewText(
				column.Source("prefix"),
				column.Dest("prefix"),
				column.Default("/"),
			),
			column.NewText(
				column.Source("main_owner"),
				column.Dest("owner"),
				column.Default("PANIC"),
				func(records map[string]any, p any) any {
					if p == nil {
						return p
					}

					userId := p.(string)

					userId = strings.TrimSpace(userId)

					var count int64

					err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM users WHERE user_id = $1", userId).Scan(&count)

					if err != nil {
						panic(err)
					}

					if count == 0 {
						ui.NotifyMsg("warning", "User not found, adding")

						var username string

						// Call http://localhost:8080/_duser/ID
						resp, err := http.Get("http://localhost:8080/_duser/" + userId)

						if err != nil {
							fmt.Println("User fetch error:", err)
							username = "Unknown User"
						}

						if resp.StatusCode != 200 {
							fmt.Println("User fetch error:", resp.StatusCode)
							username = "Unknown User"
						}

						// Read the response body
						body, err := io.ReadAll(resp.Body)

						if err != nil {
							fmt.Println("User fetch error:", err)
							username = "Unknown User"
						}

						var data struct {
							Username string `json:"username"`
						}

						// Unmarshal the response body
						err = json.Unmarshal(body, &data)

						if err != nil {
							fmt.Println("User fetch error:", err)
							username = "Unknown User"
						}

						username = data.Username

						if _, err = pool.Exec(context.Background(), "INSERT INTO users (username, user_id, api_token) VALUES ($1, $2, $3)", username, p, helpers.RandString(128)); err != nil {
							panic(err)
						}
					}

					return userId
				},
			),
			column.NewText(
				column.Source("additional_owners"),
				column.Dest("additional_owners"),
				column.ArrayJSONDefault,
			).SetArray(true),
			column.NewBool(
				column.Source("staff"),
				column.Dest("staff_bot"),
				column.Default(false),
			),
			column.NewText(
				column.Source("short"),
				column.Dest("short"),
				column.Default("PANIC"),
			),
			column.NewText(
				column.Source("long"),
				column.Dest("long"),
				column.Default("PANIC"),
			),
			column.NewText(
				column.Source("library"),
				column.Dest("library"),
				column.Default("custom"),
			),
			column.NewJSONB(
				column.Source("extra_links"),
				column.Dest("extra_links"),
				func(record map[string]any, col any) any {
					parsedLinks := []link{}
					for _, name := range []string{"website", "github", "donate", "support"} {
						linkC, ok := record[name]

						if !ok {
							continue
						}

						linkStr, ok := linkC.(string)

						if !ok {
							continue
						}

						// Title-case name
						name = cases.Title(language.AmericanEnglish).String(name)

						parsedLink := parseLink(name, linkStr)

						if parsedLink != "" {
							parsedLinks = append(parsedLinks, link{
								Name:  name,
								Value: parsedLink,
							})
						}
					}

					return parsedLinks
				},
			),
			column.NewBool(
				column.Source("nsfw"),
				column.Dest("nsfw"),
				column.Default(false),
			),
			column.NewBool(
				column.Source("premium"),
				column.Dest("premium"),
				column.Default(false),
			),
			column.NewBool(
				column.Source("pending_cert"),
				column.Dest("pending_cert"),
				column.Default(false),
			),
			column.NewBigInt(
				column.Source("servers"),
				column.Dest("servers"),
				column.Default(0),
			),
			column.NewBigInt(
				column.Source("shards"),
				column.Dest("shards"),
				column.Default(0),
			),
			column.NewBigInt(
				column.Source("users"),
				column.Dest("users"),
				column.Default(0),
			),
			column.NewInt(
				column.Source("shardArray"),
				column.Dest("shard_array"),
				column.Default(column.ArrayJSONDefault),
			).SetArray(true),
			column.NewInt(
				column.Source("votes"),
				column.Dest("votes"),
				column.Default(0),
			),
			column.NewInt(
				column.Source("clicks"),
				column.Dest("clicks"),
				column.Default(0),
			),
			column.NewInt(
				column.Source("invite_clicks"),
				column.Dest("invite_clicks"),
				column.Default(0),
			),
			column.NewText(
				column.Source("background"),
				column.Dest("banner"),
				nil,
			).SetNullable(true),
			column.NewText(
				column.Source("invite"),
				column.Dest("invite"),
				nil,
			).SetNullable(true),
			column.NewText(
				column.Source("type"),
				column.Dest("type"),
				column.Default("pending"),
				func(record map[string]any, col any) any {
					certified, ok := record["certified"].(bool)

					if certified && ok {
						return "certified"
					}

					claimed, ok := record["claimed"].(bool)

					if ok && claimed {
						return "claimed"
					}

					return col
				},
			),
			column.NewText(
				column.Source("vanity"),
				column.Dest("vanity"),
				column.Default("PANIC"),
				func(record map[string]any, col any) any {
					if col == nil {
						// Generate vanity as random string
						return helpers.RandString(8)
					}

					// Check that vanity is not taken
					var colCast = col.(string)

					var count int
					for _, v := range table.Records {
						vanity, ok := v["vanity"]

						if !ok {
							continue
						}

						vanityStr, ok := vanity.(string)

						if !ok {
							continue
						}

						if strings.EqualFold(vanityStr, colCast) {
							count++
						}
					}

					if count > 1 {
						return helpers.RandString(8)
					}

					return colCast
				},
			).SetUnique(true),
			column.NewText(
				column.Source("external_source"),
				column.Dest("external_source"),
				nil,
			).SetNullable(true),
			column.NewUUID(
				column.Source("listSource"),
				column.Dest("list_source"),
				"NULL",
			).SetNullable(true),
			column.NewBool(
				column.Source("vote_banned"),
				column.Dest("vote_banned"),
				column.Default(false),
			),
			column.NewBool(
				column.Source("cross_add"),
				column.Dest("cross_add"),
				column.Default(true),
			),
			column.NewBigInt(
				column.Source("start_period"),
				column.Dest("start_premium_period"),
				column.Default(0),
			),
			column.NewBigInt(
				column.Source("sub_period"),
				column.Dest("premium_period_length"),
				column.Default(0),
			),
			column.NewText(
				column.Source("cert_reason"),
				column.Dest("cert_reason"),
				nil,
			).SetNullable(true),
			column.NewBool(
				column.Source("announce"),
				column.Dest("announce"),
				column.Default(false),
			),
			column.NewText(
				column.Source("announce_msg"),
				column.Dest("announce_message"),
				nil,
			).SetNullable(true),
			column.NewBigInt(
				column.Source("uptime"),
				column.Dest("uptime"),
				column.Default(0),
			),
			column.NewBigInt(
				column.Source("total_uptime"),
				column.Dest("total_uptime"),
				column.Default(0),
			),
			column.NewBigInt(
				column.Source("claimedBy"),
				column.Dest("claimed_by"),
				nil,
			).SetNullable(true),
			column.NewText(
				column.Source("note"),
				column.Dest("approval_note"),
				nil,
			).SetNullable(true),
			column.NewTimestamp(
				column.Source("date"),
				column.Dest("created_at"),
				"NOW()",
				transform.ToTimestamp,
			),
			column.NewText(
				column.Source("webAuth"),
				column.Dest("web_auth"),
				nil,
			).SetNullable(true),
			column.NewText(
				column.Source("webURL"),
				column.Dest("webhook"),
				nil,
			).SetNullable(true),
			column.NewBool(
				column.Source("webHmac"),
				column.Dest("hmac"),
				column.Default(false),
			),
			column.NewText(
				column.Source("unique_clicks"),
				column.Dest("unique_clicks"),
				column.ArrayJSONDefault,
				func(record map[string]any, col any) any {
					uc, ok := record["unique_clicks"].(primitive.A)

					if !ok {
						return []string{}
					}

					var uniqueClicks []string = make([]string, len(uc))
					for i, v := range uc {
						// hash v using sha512
						h := sha512.New()

						h.Write([]byte(v.(string)))

						uniqueClicks[i] = hex.EncodeToString(h.Sum(nil))
					}

					return uniqueClicks
				},
			).SetArray(true),
			column.NewText(
				column.Source("token"),
				column.Dest("api_token"),
				nil,
			).SetSQLDefault("uuid_generate_v4()"),
			column.NewTimestamp(
				column.Source("last_claimed"),
				column.Dest("last_claimed"),
				"NULL",
				transform.ToTimestamp,
			).SetNullable(true),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName: "claims",
		DstName: "reports",
		/*
			BotID       string    `bson:"botID" json:"bot_id" unique:"true" fkey:"bots,bot_id"`
			ClaimedBy   string    `bson:"claimedBy" json:"claimed_by"`
			Claimed     bool      `bson:"claimed" json:"claimed"`
			ClaimedAt   time.Time `bson:"claimedAt" json:"claimed_at" default:"NOW()"`
			UnclaimedAt time.Time `bson:"unclaimedAt" json:"unclaimed_at" default:"NOW()"`
		*/
		Columns: column.Columns(
			column.NewText(
				column.Source("botID"),
				column.Dest("bot_id"),
				column.NoDefault,
			).SetUnique(true).SetForeignKey([2]string{"bots", "bot_id"}),
			column.NewText(
				column.Source("claimedBy"),
				column.Dest("claimed_by"),
				column.NoDefault,
			),
			column.NewBool(
				column.Source("claimed"),
				column.Dest("claimed"),
				column.Default(false),
			),
			column.NewTimestamp(
				column.Source("claimedAt"),
				column.Dest("claimed_at"),
				"NOW()",
				transform.ToTimestamp,
			),
			column.NewTimestamp(
				column.Source("unclaimedAt"),
				column.Dest("unclaimed_at"),
				"NOW()",
				transform.ToTimestamp,
			),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName: "announcements",
		DstName: "announcements",
		Columns: column.Columns(
			/*
				UserID         string    `bson:"userID" json:"user_id" fkey:"users,user_id"`
				AnnouncementID string    `bson:"announceID" json:"id" mark:"uuid" defaultfunc:"uuidgen" default:"uuid_generate_v4()" omit:"true"`
				Title          string    `bson:"title" json:"title"`
				Content        string    `bson:"content" json:"content"`
				ModifiedDate   time.Time `bson:"modifiedDate" json:"modified_date" default:"NOW()"`
				ExpiresDate    time.Time `bson:"expiresDate,omitempty" json:"expires_date" default:"NOW()"`
				Status         string    `bson:"status" json:"status" default:"'active'"`
				Targetted      bool      `bson:"targetted" json:"targetted" default:"false"`
				Target         []string  `bson:"target,omitempty" json:"target" default:"null"`
			*/
			column.NewText(
				column.Source("userID"),
				column.Dest("user_id"),
				column.NoDefault,
			).SetForeignKey([2]string{"users", "user_id"}),
			column.NewUUID(
				column.Source("announceID"),
				column.Dest("id"),
				"NULL",
			).SetSQLDefault("uuid_generate_v4()"),
			column.NewText(
				column.Source("title"),
				column.Dest("title"),
				column.NoDefault,
			),
			column.NewText(
				column.Source("content"),
				column.Dest("content"),
				column.NoDefault,
			),
			column.NewTimestamp(
				column.Source("modifiedDate"),
				column.Dest("modified_date"),
				"NOW()",
				transform.ToTimestamp,
			),
			column.NewTimestamp(
				column.Source("expiresDate"),
				column.Dest("expires_date"),
				"NOW()",
			),
			column.NewText(
				column.Source("status"),
				column.Dest("status"),
				"active",
			),
			column.NewBool(
				column.Source("targetted"),
				column.Dest("targetted"),
				false,
			),
			column.NewText(
				column.Source("target"),
				column.Dest("target"),
				column.ArrayJSONDefault,
			).SetArray(true),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName:       "votes",
		DstName:       "votes",
		IgnoreFKError: true,
		Columns: column.Columns(
			/*
							UserID string    `bson:"userID" json:"user_id" fkey:"users,user_id" fkignore:"true"`
				BotID  string    `bson:"botID" json:"bot_id" fkey:"bots,bot_id"`
				Date   time.Time `bson:"date" json:"date" default:"NOW()"`
			*/
			column.NewText(
				column.Source("userID"),
				column.Dest("user_id"),
				column.NoDefault,
			).SetForeignKey([2]string{"users", "user_id"}),
			column.NewText(
				column.Source("botID"),
				column.Dest("bot_id"),
				column.NoDefault,
			).SetForeignKey([2]string{"bots", "bot_id"}),

			column.NewTimestamp(
				column.Source("date"),
				column.Dest("date"),
				"NOW()",
				transform.ToTimestamp,
			),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName: "packages",
		DstName: "packs",
		Columns: column.Columns(
			/*
				Owner   string    `bson:"owner" json:"owner" fkey:"users,user_id"`
				Name    string    `bson:"name" json:"name" default:"'My pack'"`
				Short   string    `bson:"short" json:"short"`
				TagsRaw string    `bson:"tags" json:"tags" tolist:"true"`
				URL     string    `bson:"url" json:"url" unique:"true"`
				Date    time.Time `bson:"date" json:"date" default:"NOW()"`
				Bots    []string  `bson:"bots" json:"bots" tolist:"true"`
			*/
			column.NewText(
				column.Source("owner"),
				column.Dest("owner"),
				column.NoDefault,
			).SetForeignKey([2]string{"users", "user_id"}),
			column.NewText(
				column.Source("name"),
				column.Dest("name"),
				"My pack",
			),
			column.NewText(
				column.Source("short"),
				column.Dest("short"),
				column.NoDefault,
			),
			column.NewText(
				column.Source("tags"),
				column.Dest("tags"),
				column.ArrayJSONDefault,
				transform.ToList,
			).SetArray(true),
			column.NewText(
				column.Source("url"),
				column.Dest("url"),
				column.NoDefault,
			).SetUnique(true),
			column.NewTimestamp(
				column.Source("date"),
				column.Dest("created_at"),
				"NOW()",
				transform.ToTimestamp,
			),
			column.NewText(
				column.Source("bots"),
				column.Dest("bots"),
				column.ArrayJSONDefault,
			).SetArray(true),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName:       "reviews",
		DstName:       "reviews",
		IgnoreFKError: true,
		Columns: column.Columns(
			/*
				ID pgtype.UUID `bson:"_id" json:"id" default:"uuid_generate_v4()"`
				BotID       string         `bson:"botID" json:"bot_id" fkey:"bots,bot_id"`
				Author      string         `bson:"author" json:"author" fkey:"users,user_id"`
				Content     string         `bson:"content" json:"content" default:"'Very good bot!'"`
				StarRate    int            `bson:"star_rate" json:"stars" default:"1"`
				CreatedAt        time.Time      `bson:"date" json:"created_at" default:"NOW()"`
				Parent	  string         `bson:"parent" json:"parent" fkey:"reviews,review_id"`
			*/
			column.NewUUID(
				column.Source("id"),
				column.Dest("id"),
				"NULL",
			).SetSQLDefault("uuid_generate_v4()"),
			column.NewText(
				column.Source("botID"),
				column.Dest("bot_id"),
				column.NoDefault,
			).SetForeignKey([2]string{"bots", "bot_id"}),
			column.NewText(
				column.Source("author"),
				column.Dest("author"),
				column.NoDefault,
			).SetForeignKey([2]string{"users", "user_id"}),
			column.NewText(
				column.Source("content"),
				column.Dest("content"),
				"Very good bot!",
			),
			column.NewInt(
				column.Source("star_rate"),
				column.Dest("stars"),
				5,
			),
			column.NewTimestamp(
				column.Source("date"),
				column.Dest("created_at"),
				"NOW()",
				transform.ToTimestamp,
			),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName: "replies",
		DstName: "replies",
		Columns: column.Columns(
			column.NewUUID(
				column.Source("id"),
				column.Dest("id"),
				"NULL",
			).SetSQLDefault("uuid_generate_v4()"),
			column.NewText(
				column.Source("author"),
				column.Dest("author"),
				column.NoDefault,
			).SetForeignKey([2]string{"users", "user_id"}),
			column.NewText(
				column.Source("content"),
				column.Dest("content"),
				"Very good bot!",
			),
			column.NewInt(
				column.Source("star_rate"),
				column.Dest("star_rate"),
				5,
			),
			column.NewTimestamp(
				column.Source("date"),
				column.Dest("created_at"),
				"NOW()",
				transform.ToTimestamp,
			),
			column.NewUUID(
				column.Source("parent"),
				column.Dest("parent"),
				column.NoDefault,
			).SetForeignKey([2]string{"reviews", "id"}),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName: "tickets",
		DstName: "tickets",
		Columns: column.Columns(
			/*
				ChannelID      string    `bson:"channelID" json:"channel_id"`
				Topic          string    `bson:"topic" json:"topic" default:"'Support'"`
				UserID         string    `bson:"userID" json:"user_id"` // No fkey here bc a user may not be a user on the table yet
				TicketID       int       `bson:"ticketID" json:"id" unique:"true"`
				LogURL         string    `bson:"logURL,omitempty" json:"log_url" default:"null"`
				CloseUserID    string    `bson:"closeUserID,omitempty" json:"close_user_id" default:"null"`
				Open           bool      `bson:"open" json:"open" default:"true"`
				Date           time.Time `bson:"date" json:"date" default:"NOW()"`
				PanelMessageID string    `bson:"panelMessageID,omitempty" json:"panel_message_id" default:"null"`
				PanelChannelID string    `bson:"panelChannelID,omitempty" json:"panel_channel_id" default:"null"`
			*/

			column.NewText(
				column.Source("channelID"),
				column.Dest("channel_id"),
				column.NoDefault,
			),
			column.NewText(
				column.Source("topic"),
				column.Dest("topic"),
				"Support",
			),
			column.NewText(
				column.Source("userID"),
				column.Dest("user_id"),
				column.NoDefault,
			),
			column.NewInt(
				column.Source("ticketID"),
				column.Dest("id"),
				column.NoDefault,
			).SetUnique(true),
			column.NewText(
				column.Source("logURL"),
				column.Dest("log_url"),
				"NULL",
			).SetNullable(true),
			column.NewText(
				column.Source("closeUserID"),
				column.Dest("close_user_id"),
				"NULL",
			).SetNullable(true),
			column.NewBool(
				column.Source("open"),
				column.Dest("open"),
				true,
			),
			column.NewTimestamp(
				column.Source("date"),
				column.Dest("created_at"),
				"NOW()",
				transform.ToTimestamp,
			),
			column.NewText(
				column.Source("panelMessageID"),
				column.Dest("panel_message_id"),
				"NULL",
			).SetNullable(true),
			column.NewText(
				column.Source("panelChannelID"),
				column.Dest("panel_channel_id"),
				"NULL",
			).SetNullable(true),
		),
	}.Migrate(source, pool)

	table.Table{
		SrcName: "transcripts",
		DstName: "transcripts",
		Columns: column.Columns(
		/*
			TicketID int            `bson:"ticketID" json:"id" fkey:"tickets,id"`
			Data     map[string]any `bson:"data" json:"data" default:"{}"`
			ClosedBy map[string]any `bson:"closedBy" json:"closed_by" default:"{}"`
			OpenedBy map[string]any `bson:"openedBy" json:"opened_by" default:"{}"`
		*/
		),
	}.Migrate(source, pool)
}

// Custom transform helpers
func parseLink(key string, link string) string {
	if strings.HasPrefix(link, "http://") {
		return strings.Replace(link, "http://", "https://", 1)
	}

	if strings.HasPrefix(link, "https://") {
		ui.NotifyMsg("debug", "Keeping link "+link)
		return link
	}

	ui.NotifyMsg("debug", "Possibly Invalid URL found: "+link)

	if key == "Support" && !strings.Contains(link, " ") {
		link = strings.Replace(link, "www", "", 1)
		if strings.HasPrefix(link, "discord.gg/") {
			link = "https://discord.gg/" + link[11:]
		} else if strings.HasPrefix(link, "discord.com/invite/") {
			link = "https://discord.gg/" + link[19:]
		} else if strings.HasPrefix(link, "discord.com/") {
			link = "https://discord.gg/" + link[12:]
		} else {
			link = "https://discord.gg/" + link
		}
		ui.NotifyMsg("debug", "Succesfully fixed support link to"+link)
		return link
	} else {
		// But wait, it may be safe still
		split := strings.Split(link, "/")[0]
		tldLst := strings.Split(split, ".")

		if len(tldLst) > 1 && (len(tldLst[len(tldLst)-1]) == 2 || slices.Contains([]string{
			"com",
			"net",
			"org",
			"fun",
			"app",
			"dev",
			"xyz",
		}, tldLst[len(tldLst)-1])) {
			ui.NotifyMsg("debug", "Fixed found URL link to https://"+link)
			return "https://" + link
		} else {
			if strings.HasPrefix(link, "https://") {
				return link
			}

			ui.NotifyMsg("warning", "Removing invalid link: "+link)
			return ""
		}
	}
}

type link struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
