package filter

// source https://api.sportmonks.com/v3/football/leagues?include=country:name&per_page=100
var leagueNameToID = map[string]int{ // league name should be in lowercase
	"premier league":                       8,
	"english premier league":               8,
	"english football league championship": 9,
	"english championship":                 9,
	"championship":                         9,
	"fa cup":                               24,
	"carabao cup":                          27,
	"dutch eredivisie":                     72,
	"eredivisie":                           72,
	"german bundesliga":                    82,
	"bundesliga":                           82,
	"austrian bundesliga":                  181,
	"austrian football bundesliga":         181,
	"belgian pro league":                   208,
	"pro league":                           208,
	"1. hnl":                               244,
	"croatian football league":             244,
	"superliga":                            271,
	"french ligue 1":                       301,
	"ligue 1":                              301,
	"serie a":                              384,
	"italian serie a":                      384,
	"serie b":                              387,
	"coppa italia":                         390,
	"norwegian eliteserien":                444,
	"eliteserien":                          444,
	"polish ekstraklasa":                   453,
	"ekstraklasa":                          453,
	"liga portugal betclic":                462,
	"primeira liga":                        462,
	"portuguese primeira liga":             462,
	"liga portugal":                        462,
	"scottish premiership":                 501,
	"premiership":                          501,
	"spanish la liga":                      564,
	"la liga 2":                            567,
	"copa del rey":                         570,
	"allsvenskan":                          573,
	"swiss super league":                   591,
	"super league":                         591,
	"turkish süper lig":                    600,
	"super lig":                            600,
	"ukrainian premier league":             609,
	"ukraine premier league":               609,
	"uefa europa league play-offs":         1371,
	"russian premier league":               0, // 486 no such country
}
