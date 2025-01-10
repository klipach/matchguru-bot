package matchguru

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	leagueRegexp = regexp.MustCompile(`<l fullname="([^"]+)">([^<]+)</l>`)
)

func process(text string) string {
	leagues := map[string]int{
		"Premier League England":           609,
		"English Premier League":           609,
		"EFL Championship England":         9,
		"FA Cup":                           24,
		"Carabao Cup":                      27,
		"Eredivisie Netherlands":           72,
		"Bundesliga":                       82,
		"Bundesliga Germany":               82,
		"German Bundesliga":                82,
		"Austrian Bundesliga Austria":      181,
		"Austrian Bundesliga":              181,
		"Jupiler Pro League Belgium":       208,
		"Belgian Pro League Belgium":       208,
		"Prva HNL Croatia":                 244,
		"Danish Superliga Denmark":         271,
		"French Ligue 1":                   301,
		"Ligue 1 France":                   301,
		"Italian Serie A":                  384,
		"Serie A Italy":                    384,
		"Serie B Italy":                    387,
		"Coppa Italia":                     390,
		"Eliteserien":                      444,
		"Ekstraklasa":                      453,
		"Primeira Liga Portugal":           462,
		"Portuguese Primeira Liga":         462,
		"Scottish Premiership Scotland":    501,
		"La Liga Spain":                    564,
		"Spanish La Liga":                  564,
		"La Liga 2":                        567,
		"Copa Del Rey":                     570,
		"Allsvenskan Sweden":               573,
		"Swiss Super League Switzerland":   591,
		"Super Lig":                        600,
		"Super Lig Turkey":                 600,
		"Ukrainian Premier League Ukraine": 609,
		"Premier League Ukraine":           609,
		"Ukrainian Premier League":         609,
		"UEFA Europa League Play-offs":     1371,
	}

	leaguesLower := make(map[string]int)
	for name, id := range leagues {
		leaguesLower[strings.ToLower(name)] = id
	}

	return leagueRegexp.ReplaceAllStringFunc(text, func(match string) string {
		submatches := leagueRegexp.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		fullname := strings.ToLower(submatches[1])
		name := submatches[2]

		if id, ok := leaguesLower[fullname]; ok {
			return fmt.Sprintf(`<a href="/league/%d">%s</a>`, id, name)
		}
		return name
	})
}
