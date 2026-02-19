package main

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// dutchMunicipalities is the complete list of Dutch gemeenten (2025 CBS), sorted alphabetically
// using standard Go string comparison (apostrophe sorts before A).
var dutchMunicipalities = []string{
	"'s-Gravenhage", "'s-Hertogenbosch",
	"Aalsmeer", "Aalten", "Achtkarspelen", "Alblasserdam", "Albrandswaard",
	"Alkmaar", "Almelo", "Almere", "Alphen aan den Rijn", "Alphen-Chaam",
	"Altena", "Ameland", "Amersfoort", "Amstelveen", "Amsterdam",
	"Apeldoorn", "Arnhem", "Assen", "Asten", "Baarle-Nassau",
	"Barendrecht", "Barneveld", "Berg en Dal", "Bergeijk", "Bergen (Limburg)",
	"Bergen (Noord-Holland)", "Bergen op Zoom", "Berkelland", "Beuningen", "Beverwijk",
	"Bladel", "Blaricum", "Bloemendaal", "Bodegraven-Reeuwijk", "Boekel",
	"Borger-Odoorn", "Borne", "Borsele", "Boxtel", "Breda",
	"Bronckhorst", "Brummen", "Brunssum", "Bunnik", "Bunschoten",
	"Buren", "Capelle aan den IJssel", "Castricum", "Coevorden", "Cranendonck",
	"Culemborg", "Dalfsen", "Dantumadiel", "De Bilt", "De Fryske Marren",
	"De Ronde Venen", "De Wolden", "Delft", "Den Helder", "Deurne",
	"Deventer", "Diemen", "Dijk en Waard", "Dinkelland", "Doesburg",
	"Doetinchem", "Dongen", "Dordrecht", "Drechterland", "Drimmelen",
	"Dronten", "Druten", "Duiven", "Ede", "Edam-Volendam",
	"Eemsdelta", "Eemnes", "Eersel", "Eijsden-Margraten", "Eindhoven",
	"Emmen", "Enkhuizen", "Enschede", "Epe", "Ermelo",
	"Etten-Leur", "Geertruidenberg", "Geldrop-Mierlo", "Gemert-Bakel", "Gennep",
	"Gilze en Rijen", "Goes", "Goirle", "Gooise Meren", "Gorinchem",
	"Gouda", "Groningen", "Gulpen-Wittem", "Haaksbergen", "Haarlem",
	"Haarlemmermeer", "Hardenberg", "Harderwijk", "Hardinxveld-Giessendam", "Harlingen",
	"Hattem", "Heemskerk", "Heemstede", "Heerde", "Heerenveen",
	"Heerlen", "Heeze-Leende", "Hellendoorn", "Hellevoetsluis", "Helmond",
	"Hendrik-Ido-Ambacht", "Het Hogeland", "Heumen", "Heusden", "Hillegom",
	"Hilvarenbeek", "Hilversum", "Hoeksche Waard", "Hof van Twente", "Hollands Kroon",
	"Hoogeveen", "Hoorn", "Horst aan de Maas", "Houten", "Huizen",
	"Hulst", "IJsselstein", "Kaag en Braassem", "Kampen", "Kapelle",
	"Katwijk", "Koggenland", "Krimpen aan den IJssel", "Krimpenerwaard", "Laarbeek",
	"Land van Cuijk", "Landgraaf", "Landsmeer", "Langedijk", "Lansingerland",
	"Laren", "Leeuwarden", "Leiden", "Leiderdorp", "Leidschendam-Voorburg",
	"Lelystad", "Leudal", "Lingewaard", "Lisse", "Lochem",
	"Loon op Zand", "Lopik", "Losser", "Maasdriel", "Maasgouw",
	"Maashorst", "Maastricht", "Medemblik", "Meijerijstad", "Middelburg",
	"Midden-Delfland", "Midden-Groningen", "Moerdijk", "Molenlanden", "Montferland",
	"Montfoort", "Mook en Middelaar", "Neder-Betuwe", "Nieuwegein", "Nieuwkoop",
	"Nijkerk", "Nijmegen", "Nissewaard", "Noord-Beveland", "Noardeast-Fryslân",
	"Noordenveld", "Noordoostpolder", "Noordwijk", "Nunspeet", "Oegstgeest",
	"Oisterwijk", "Oldambt", "Oldebroek", "Olst-Wijhe", "Ommen",
	"Oost Gelre", "Oosterhout", "Ooststellingwerf", "Oostzaan", "Opmeer",
	"Opsterland", "Oss", "Oude IJsselstreek", "Ouder-Amstel", "Oudewater",
	"Overbetuwe", "Papendrecht", "Peel en Maas", "Pekela", "Pijnacker-Nootdorp",
	"Purmerend", "Putten", "Raalte", "Reimerswaal", "Renkum",
	"Reusel-De Mierden", "Rheden", "Rhenen", "Ridderkerk", "Rijssen-Holten",
	"Rijswijk", "Roerdalen", "Roermond", "Roosendaal", "Rotterdam",
	"Rozendaal", "Rucphen", "Schagen", "Scherpenzeel", "Schiedam",
	"Schouwen-Duiveland", "Simpelveld", "Sint-Michielsgestel", "Sittard-Geleen", "Sliedrecht",
	"Sluis", "Smallingerland", "Soest", "Someren", "Son en Breugel",
	"Stadskanaal", "Staphorst", "Stede Broec", "Steenbergen", "Steenwijkerland",
	"Stein (L)", "Stichtse Vecht", "Súdwest-Fryslân", "Teylingen", "Tholen",
	"Tilburg", "Tubbergen", "Twenterand", "Tynaarlo", "Tytsjerksteradiel",
	"Uitgeest", "Uithoorn", "Urk", "Utrecht", "Utrechtse Heuvelrug",
	"Vaals", "Valkenburg aan de Geul", "Valkenswaard", "Veendam", "Veenendaal",
	"Veere", "Veldhoven", "Velsen", "Venlo", "Venray",
	"Vijfheerenlanden", "Vlaardingen", "Vlissingen", "Voerendaal", "Voorschoten",
	"Vught", "Waadhoeke", "Waalre", "Waalwijk", "Waddinxveen",
	"Wageningen", "Wassenaar", "Waterland", "Weert", "Westervoort",
	"Westerwolde", "Westland", "Weststellingwerf", "Wierden", "Wijchen",
	"Wijdemeren", "Wijk bij Duurstede", "Winterswijk", "Woensdrecht", "Woerden",
	"Wormerland", "Woudenberg", "Zaanstad", "Zaltbommel", "Zandvoort",
	"Zeewolde", "Zeist", "Zevenaar", "Zoetermeer", "Zoeterwoude",
	"Zuidplas", "Zundert", "Zutphen", "Zwartewaterland", "Zwolle",
}

// placeToMunicipality maps lowercase woonplaats (place) names to their parent gemeente.
// Every municipality name also maps to itself (lowercased).
var placeToMunicipality = map[string]string{
	// Velsen
	"ijmuiden":                "Velsen",
	"santpoort":               "Velsen",
	"santpoort-noord":         "Velsen",
	"santpoort-zuid":          "Velsen",
	"velsen-noord":            "Velsen",
	"velsen-zuid":             "Velsen",
	"driehuis":                "Velsen",
	"velserbroek":             "Velsen",
	"velsen":                  "Velsen",

	// Amsterdam
	"amsterdam":               "Amsterdam",

	// Amstelveen
	"amstelveen":              "Amstelveen",

	// Ouder-Amstel
	"ouder-amstel":            "Ouder-Amstel",
	"duivendrecht":            "Ouder-Amstel",

	// Diemen
	"diemen":                  "Diemen",

	// Haarlem
	"haarlem":                 "Haarlem",

	// Heemstede
	"heemstede":               "Heemstede",

	// Bloemendaal
	"bloemendaal":             "Bloemendaal",
	"aerdenhout":              "Bloemendaal",
	"overveen":                "Bloemendaal",
	"bentveld":                "Bloemendaal",
	"vogelenzang":             "Bloemendaal",

	// Heemskerk
	"heemskerk":               "Heemskerk",

	// Beverwijk
	"beverwijk":               "Beverwijk",
	"wijk aan zee":            "Beverwijk",

	// Zandvoort
	"zandvoort":               "Zandvoort",

	// Den Haag / 's-Gravenhage
	"'s-gravenhage":           "'s-Gravenhage",
	"den haag":                "'s-Gravenhage",
	"scheveningen":            "'s-Gravenhage",
	"loosduinen":              "'s-Gravenhage",

	// Rijswijk
	"rijswijk":                "Rijswijk",

	// Wassenaar
	"wassenaar":               "Wassenaar",

	// Leidschendam-Voorburg
	"leidschendam":            "Leidschendam-Voorburg",
	"voorburg":                "Leidschendam-Voorburg",
	"leidschendam-voorburg":   "Leidschendam-Voorburg",

	// Zoetermeer
	"zoetermeer":              "Zoetermeer",

	// Delft
	"delft":                   "Delft",

	// Rotterdam
	"rotterdam":               "Rotterdam",

	// Schiedam
	"schiedam":                "Schiedam",

	// Vlaardingen
	"vlaardingen":             "Vlaardingen",

	// Capelle aan den IJssel
	"capelle aan den ijssel":  "Capelle aan den IJssel",

	// Krimpen aan den IJssel
	"krimpen aan den ijssel":  "Krimpen aan den IJssel",

	// Utrecht
	"utrecht":                 "Utrecht",

	// Nieuwegein
	"nieuwegein":              "Nieuwegein",

	// IJsselstein
	"ijsselstein":             "IJsselstein",

	// Houten
	"houten":                  "Houten",

	// Zeist
	"zeist":                   "Zeist",
	"den dolder":              "Zeist",
	"huis ter heide":          "Zeist",

	// De Bilt
	"de bilt":                 "De Bilt",
	"bilthoven":               "De Bilt",

	// Eindhoven
	"eindhoven":               "Eindhoven",

	// Helmond
	"helmond":                 "Helmond",

	// Veldhoven
	"veldhoven":               "Veldhoven",

	// Son en Breugel
	"son":                     "Son en Breugel",
	"breugel":                 "Son en Breugel",
	"son en breugel":          "Son en Breugel",

	// Groningen
	"groningen":               "Groningen",

	// Maastricht
	"maastricht":              "Maastricht",

	// Sittard-Geleen
	"sittard":                 "Sittard-Geleen",
	"geleen":                  "Sittard-Geleen",
	"sittard-geleen":          "Sittard-Geleen",

	// Leiden
	"leiden":                  "Leiden",

	// Leiderdorp
	"leiderdorp":              "Leiderdorp",

	// Oegstgeest
	"oegstgeest":              "Oegstgeest",

	// Voorschoten
	"voorschoten":             "Voorschoten",

	// Zoeterwoude
	"zoeterwoude":             "Zoeterwoude",

	// Katwijk
	"katwijk":                 "Katwijk",
	"rijnsburg":               "Katwijk",
	"valkenburg (zh)":         "Katwijk",

	// Dordrecht
	"dordrecht":               "Dordrecht",

	// Papendrecht
	"papendrecht":             "Papendrecht",

	// Sliedrecht
	"sliedrecht":              "Sliedrecht",

	// Arnhem
	"arnhem":                  "Arnhem",

	// Nijmegen
	"nijmegen":                "Nijmegen",

	// Ede
	"ede":                     "Ede",

	// Wageningen
	"wageningen":              "Wageningen",

	// Renkum
	"renkum":                  "Renkum",
	"oosterbeek":              "Renkum",

	// Tilburg
	"tilburg":                 "Tilburg",

	// Breda
	"breda":                   "Breda",

	// Roosendaal
	"roosendaal":              "Roosendaal",

	// Zwolle
	"zwolle":                  "Zwolle",

	// Deventer
	"deventer":                "Deventer",

	// Almelo
	"almelo":                  "Almelo",

	// Enschede
	"enschede":                "Enschede",

	// Alkmaar
	"alkmaar":                 "Alkmaar",

	// Hoorn
	"hoorn":                   "Hoorn",

	// Den Helder
	"den helder":              "Den Helder",

	// Purmerend
	"purmerend":               "Purmerend",

	// Zaanstad
	"zaandam":                 "Zaanstad",
	"zaanstad":                "Zaanstad",
	"koog aan de zaan":        "Zaanstad",
	"krommenie":               "Zaanstad",
	"wormerveer":              "Zaanstad",
	"assendelft":              "Zaanstad",
	"westzaan":                "Zaanstad",

	// Almere
	"almere":                  "Almere",

	// Lelystad
	"lelystad":                "Lelystad",

	// Dronten
	"dronten":                 "Dronten",

	// Urk
	"urk":                     "Urk",

	// Kampen
	"kampen":                  "Kampen",

	// Middelburg
	"middelburg":              "Middelburg",

	// Vlissingen
	"vlissingen":              "Vlissingen",

	// Goes
	"goes":                    "Goes",

	// Nissewaard
	"spijkenisse":             "Nissewaard",
	"nissewaard":              "Nissewaard",

	// Leeuwarden
	"leeuwarden":              "Leeuwarden",

	// Assen
	"assen":                   "Assen",

	// Emmen
	"emmen":                   "Emmen",

	// Apeldoorn
	"apeldoorn":               "Apeldoorn",

	// Doetinchem
	"doetinchem":              "Doetinchem",

	// Zutphen
	"zutphen":                 "Zutphen",

	// 's-Hertogenbosch
	"'s-hertogenbosch":        "'s-Hertogenbosch",
	"den bosch":               "'s-Hertogenbosch",
}

// lookupMunicipality returns the gemeente for a given woonplaats name, case-insensitively.
// Returns the input as-is if no mapping found.
func lookupMunicipality(placeName string) string {
	if placeName == "" {
		return ""
	}
	if muni, ok := placeToMunicipality[strings.ToLower(placeName)]; ok {
		return muni
	}
	return placeName
}

// isValidMunicipality reports whether name is a known Dutch gemeente, case-insensitively.
func isValidMunicipality(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	for _, m := range dutchMunicipalities {
		if strings.ToLower(m) == lower {
			return true
		}
	}
	return false
}

// municipalityList returns a sorted copy of the Dutch municipality list.
func municipalityList() []string {
	out := make([]string, len(dutchMunicipalities))
	copy(out, dutchMunicipalities)
	sort.Strings(out)
	return out
}

func (a *App) municipalitiesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, municipalityList())
}
