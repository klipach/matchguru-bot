package matchguru

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	leagueRegexp = regexp.MustCompile(`<l id="([^"]+)">([^<]+)</l>`)
)

func process(text string) string {
	var transfermarktToSportmonksID = map[int]int{
		8:    310,  // English Premier League
		9:    313,  // English EFL Championship
		24:   3290, // FA Cup
		27:   3300, // Carabao Cup
		72:   314,  // Eredivisie
		82:   3500, // German Bundesliga
		181:  507,  // Austrian Bundesliga
		208:  608,  // Belgian Pro League
		244:  591,  // Prva HNL (Croatia)
		271:  3434, // Danish Superliga
		301:  331,  // Ligue 1 (France)
		384:  599,  // Serie A (Italy)
		387:  605,  // Serie B (Italy)
		390:  620,  // Coppa Italia
		444:  442,  // Eliteserien (Norway)
		453:  388,  // Ekstraklasa (Poland)
		462:  3362, // Primeira Liga (Portugal)
		501:  5011, // Scottish Premiership
		564:  3372, // La Liga (Spain)
		567:  3373, // La Liga 2 (Spain)
		570:  3374, // Copa del Rey (Spain)
		573:  3401, // Allsvenskan (Sweden)
		591:  3441, // Swiss Super League
		600:  3499, // Turkish Süper Lig
		609:  306,  // Ukrainian Premier
		1371: 1041, // UEFA Europa League Play-offs
	}

	return leagueRegexp.ReplaceAllStringFunc(text, func(match string) string {
		submatches := leagueRegexp.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		transfermarktIDStr := submatches[1]
		name := submatches[2]
		transfermarktID, err := strconv.Atoi(transfermarktIDStr)
		if err != nil {
			return name
		}

		if id, ok := transfermarktToSportmonksID[transfermarktID]; ok {
			return fmt.Sprintf(`<a href="/league/%d">%s</a>`, id, name)
		}
		return name
	})
}
