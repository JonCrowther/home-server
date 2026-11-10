package nhl

// teamSlugs maps each NHL team's three letter abbreviation (as used by the NHL
// API, e.g. "EDM") to its teampalettes.com slug, for use with TeamColours.
var teamSlugs = map[string]string{
	"ANA": "anaheim-ducks",
	"BOS": "boston-bruins",
	"BUF": "buffalo-sabres",
	"CAR": "carolina-hurricanes",
	"CBJ": "columbus-blue-jackets",
	"CGY": "calgary-flames",
	"CHI": "chicago-blackhawks",
	"COL": "colorado-avalanche",
	"DAL": "dallas-stars",
	"DET": "detroit-red-wings",
	"EDM": "edmonton-oilers",
	"FLA": "florida-panthers",
	"LAK": "los-angeles-kings",
	"MIN": "minnesota-wild",
	"MTL": "montreal-canadiens",
	"NJD": "new-jersey-devils",
	"NSH": "nashville-predators",
	"NYI": "new-york-islanders",
	"NYR": "new-york-rangers",
	"OTT": "ottawa-senators",
	"PHI": "philadelphia-flyers",
	"PIT": "pittsburgh-penguins",
	"SEA": "seattle-kraken",
	"SJS": "san-jose-sharks",
	"STL": "st-louis-blues",
	"TBL": "tampa-bay-lightning",
	"TOR": "toronto-maple-leafs",
	// teampalettes.com has no Utah page yet, so TeamColours returns an error
	// for this slug until one is added.
	"UTA": "utah-mammoth",
	"VAN": "vancouver-canucks",
	"VGK": "vegas-golden-knights",
	"WPG": "winnipeg-jets",
	"WSH": "washington-capitals",
}
