package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"pouncecat/column"
	"pouncecat/helpers"
	"pouncecat/source/mongo"
	"pouncecat/table"
	"pouncecat/ui"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
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
	source := mongo.MongoSource{
		ConnectionURL:  os.Getenv("MONGO"),
		DatabaseName:   "infinity",
		IgnoreEntities: []string{"sessions"},
	}

	err := source.Connect()

	if err != nil {
		panic(err)
	}

	pool, err := pgxpool.New(context.Background(), "postgresql://127.0.0.1:5432/infinity?user=root&password=iblpublic")

	if err != nil {
		panic(err)
	}

	table.PrepareTables(pool)

	table.Table{
		SrcName: "users",
		DstName: "users",
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
				nil,
			).SetNullable(true),
			column.NewTimestamp(
				column.Source("staff_onboard_macro_time"),
				column.Dest("staff_onboard_macro_time"),
				nil,
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
				map[string]any{},
				func(record map[string]any, col any) any {
					parsedLinks := map[string]string{}
					for _, name := range []string{"website", "github"} {
						link, ok := record[name]

						if !ok {
							continue
						}

						linkStr, ok := link.(string)

						if !ok {
							continue
						}

						// Title-case name
						name = cases.Title(language.AmericanEnglish).String(name)

						parsedLink := parseLink(name, linkStr)

						if parsedLink != "" {
							parsedLinks[name] = parsedLink
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
