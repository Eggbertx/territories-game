package svgmap

import (
	"bytes"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/antchfx/xmlquery"
)

var (
	fillRE = regexp.MustCompile(`(.*fill:\s*#)([0-9a-fA-F]{6})(.*)`)
)

func openXMLDoc(file string) (*xmlquery.Node, error) {
	ba, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return xmlquery.Parse(bytes.NewReader(ba))
}

func svgDocToPNG(doc *xmlquery.Node, out string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get configuration: %w", err)
	}

	if err = os.WriteFile(cfg.SVGOutFile, []byte(doc.OutputXML(true)), 0644); err != nil {
		return err
	}

	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-i", cfg.SVGOutFile, out)
	var ffmpegLogBuf bytes.Buffer
	cmd.Stdout = &ffmpegLogBuf
	cmd.Stderr = &ffmpegLogBuf
	if err = cmd.Run(); err != nil {
		os.WriteFile("ffmpeg.log", ffmpegLogBuf.Bytes(), 0644)
		return fmt.Errorf("ffmpeg command failed: %w\n%s", err, ffmpegLogBuf.String())
	}
	return nil
}

func updateStateColorWorker(doc *xmlquery.Node, state, newColor string) error {
	node := xmlquery.FindOne(doc, fmt.Sprintf("//path[@id=%q]", state))
	if node == nil {
		return fmt.Errorf("path not found with id %q", state)
	}
	style := node.SelectAttr("style")
	var matches [][]string
	if style != "" {
		// style = "fill:#" + newColor + ";"
		matches = fillRE.FindAllStringSubmatch(style, 1)
	}
	if matches == nil {
		style = "fill:#" + newColor + ";" + style
	} else {
		style = matches[0][1] + newColor + matches[0][3]
	}
	node.SetAttr("style", style)
	return nil
}

func batchUpdateStateColors(changes []db.HoldingRecord) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get configuration: %w", err)
	}

	tdb, err := db.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	doc, err := openXMLDoc(cfg.MapFile)
	if err != nil {
		return err
	}

	if err = updateCountryList(doc, tdb); err != nil {
		return err
	}

	if err = updateTerritoryArmies(tdb, doc); err != nil {
		return err
	}

	for _, change := range changes {
		if err = updateStateColorWorker(doc, change.Territory, change.Color); err != nil {
			return err
		}
	}
	return svgDocToPNG(doc, cfg.PNGOutFile)
}

func updateCountryList(doc *xmlquery.Node, tdb *sql.DB) error {
	nationsListGroup := xmlquery.FindOne(doc, "//g[@id='nations-list']")
	if nationsListGroup == nil {
		return fmt.Errorf("nations-list g element not found in SVG document")
	}

	nationsListBoundsRect := xmlquery.FindOne(nationsListGroup, "//rect[@id='nations-list-bounds']")
	if nationsListBoundsRect == nil {
		return fmt.Errorf("nations-list-bounds rect element not found in SVG document")
	}

	// Remove placeholder nodes
	nodesToDelete := xmlquery.Find(nationsListGroup, "//text[@class='nation-name']")
	nodesToDelete = append(nodesToDelete, xmlquery.Find(nationsListGroup, "//rect[@class='nation-color']")...)
	for _, node := range nodesToDelete {
		xmlquery.RemoveFromTree(node)
	}

	boundsXStr := nationsListBoundsRect.SelectAttr("x")
	boundsX, err := strconv.ParseFloat(boundsXStr, 64)
	if err != nil {
		return fmt.Errorf("invalid x attribute in nations-list-bounds rect: %v", err)
	}

	boundsYStr := nationsListBoundsRect.SelectAttr("y")
	boundsY, err := strconv.ParseFloat(boundsYStr, 64)
	if err != nil {
		return fmt.Errorf("invalid y attribute in nations-list-bounds rect: %v", err)
	}

	rows, err := tdb.Query("SELECT country_name, color, player FROM nations")
	if err != nil {
		return err
	}
	defer rows.Close()

	nationIndex := 1
	var countryName, color, player string
	for rows.Next() {
		if err = rows.Scan(&countryName, &color, &player); err != nil {
			return err
		}

		textNode := &xmlquery.Node{
			Type: xmlquery.ElementNode,
			Data: "text",
			Attr: []xmlquery.Attr{
				{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("nation-name-%d", nationIndex)},
				{Name: xml.Name{Local: "class"}, Value: "nation-name"},
				{Name: xml.Name{Local: "x"}, Value: strconv.Itoa(int(boundsX + 46))},
				{Name: xml.Name{Local: "y"}, Value: strconv.Itoa(int(boundsY) + 32*nationIndex)},
			},
			FirstChild: &xmlquery.Node{
				Type: xmlquery.TextNode,
				Data: fmt.Sprintf("%s (leader: %s)", countryName, player),
			},
		}
		xmlquery.AddChild(nationsListGroup, textNode)

		rectNode := &xmlquery.Node{
			Type: xmlquery.ElementNode,
			Data: "rect",
			Attr: []xmlquery.Attr{
				{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("nation-color-%d", nationIndex)},
				{Name: xml.Name{Local: "class"}, Value: "nation-color"},
				{Name: xml.Name{Local: "style"}, Value: fmt.Sprintf("fill:#%s", color)},
				{Name: xml.Name{Local: "width"}, Value: "28"},
				{Name: xml.Name{Local: "height"}, Value: "28"},
				{Name: xml.Name{Local: "x"}, Value: strconv.Itoa(int(boundsX + 10))},
				{Name: xml.Name{Local: "y"}, Value: strconv.Itoa(int(boundsY) + 32*nationIndex - 20)},
			},
		}
		xmlquery.AddChild(nationsListGroup, rectNode)
		nationIndex++
	}
	return rows.Close()
}

func addCircle(parent *xmlquery.Node, id string, class string, cx, cy, r float64, style string) *xmlquery.Node {
	circle := &xmlquery.Node{
		Type: xmlquery.ElementNode,
		Data: "circle",
		Attr: []xmlquery.Attr{
			{Name: xml.Name{Local: "id"}, Value: id},
			{Name: xml.Name{Local: "class"}, Value: class},
			{Name: xml.Name{Local: "cx"}, Value: fmt.Sprintf("%f", cx)},
			{Name: xml.Name{Local: "cy"}, Value: fmt.Sprintf("%f", cy)},
			{Name: xml.Name{Local: "r"}, Value: fmt.Sprintf("%f", r)},
			{Name: xml.Name{Local: "style"}, Value: style},
		},
	}
	xmlquery.AddChild(parent, circle)
	return circle
}

func updateTerritoryArmies(db *sql.DB, doc *xmlquery.Node) error {
	armiesContainer := xmlquery.FindOne(doc, "//g[@id='armies-container']")
	if armiesContainer == nil {
		return fmt.Errorf("armies-container g element not found in SVG document")
	}
	const armyCircleStyle = "fill:green;stroke:black;stroke-width:2"

	rows, err := db.Query(`SELECT territory, army_size FROM holdings`)
	if err != nil {
		return fmt.Errorf("failed to query holdings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var armies int
		var territory string
		if err := rows.Scan(&territory, &armies); err != nil {
			return fmt.Errorf("failed to scan holding record: %w", err)
		}

		if armies == 0 {
			continue // No armies on this territory
		}

		// TODO: get this from the database instead of the config
		armyPlaceholder := xmlquery.FindOne(armiesContainer, fmt.Sprintf("//circle[@id=%q]", territory+"-armies"))
		if armyPlaceholder == nil {
			return fmt.Errorf("army placeholder not found for territory %q", territory)
		}
		radiusStr := armyPlaceholder.SelectAttr("r")
		radius, err := strconv.ParseFloat(radiusStr, 64)
		if err != nil {
			return fmt.Errorf("invalid radius attribute for army placeholder in territory %q: %v", territory, err)
		}
		cxStr := armyPlaceholder.SelectAttr("cx")
		cx, err := strconv.ParseFloat(cxStr, 64)
		if err != nil {
			return fmt.Errorf("invalid cx attribute for army placeholder in territory %q: %v", territory, err)
		}
		cyStr := armyPlaceholder.SelectAttr("cy")
		cy, err := strconv.ParseFloat(cyStr, 64)
		if err != nil {
			return fmt.Errorf("invalid cy attribute for army placeholder in territory %q: %v", territory, err)
		}

		armyCircleSize := radius / 3
		switch armies {
		case 1:
			addCircle(armiesContainer, fmt.Sprintf("%s-army-1", territory), "army", cx, cy, armyCircleSize, armyCircleStyle)
		case 2:
			addCircle(armiesContainer, fmt.Sprintf("%s-army-1", territory), "army", cx-armyCircleSize, cy, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-2", territory), "army", cx+armyCircleSize, cy, armyCircleSize, armyCircleStyle)
		case 3:
			addCircle(armiesContainer, fmt.Sprintf("%s-army-1", territory), "army", cx-armyCircleSize, cy-armyCircleSize, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-2", territory), "army", cx+armyCircleSize, cy-armyCircleSize, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-3", territory), "army", cx, cy+armyCircleSize, armyCircleSize, armyCircleStyle)
		case 4:
			addCircle(armiesContainer, fmt.Sprintf("%s-army-1", territory), "army", cx-armyCircleSize, cy-armyCircleSize, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-2", territory), "army", cx+armyCircleSize, cy-armyCircleSize, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-3", territory), "army", cx-armyCircleSize, cy+armyCircleSize, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-4", territory), "army", cx+armyCircleSize, cy+armyCircleSize, armyCircleSize, armyCircleStyle)
		case 5:
			addCircle(armiesContainer, fmt.Sprintf("%s-army-1", territory), "army", cx-armyCircleSize*1.5, cy-armyCircleSize*1.5, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-2", territory), "army", cx+armyCircleSize*1.5, cy-armyCircleSize*1.5, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-3", territory), "army", cx-armyCircleSize*1.5, cy+armyCircleSize*1.5, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-4", territory), "army", cx+armyCircleSize*1.5, cy+armyCircleSize*1.5, armyCircleSize, armyCircleStyle)
			addCircle(armiesContainer, fmt.Sprintf("%s-army-5", territory), "army", cx, cy, armyCircleSize, armyCircleStyle)
		default:
			return fmt.Errorf("unexpected number of armies: %d for territory %s", armies, territory)
		}
	}
	return nil
}

func ApplyDBEvents() error {
	tdb, err := db.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	rows, err := tdb.Query(`SELECT territory, army_size, color, country_name FROM v_nation_holdings`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var records []db.HoldingRecord
	for rows.Next() {
		var record db.HoldingRecord
		if err = rows.Scan(&record.Territory, &record.ArmySize, &record.Color, &record.CountryName); err != nil {
			return err
		}
		records = append(records, record)
	}
	if err = rows.Close(); err != nil {
		return err
	}

	return batchUpdateStateColors(records)
}

func ValidateMap() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}
	if cfg.MapFile == "" {
		return errors.New("map file is not specified")
	}
	if cfg.MaxArmiesPerTerritory <= 0 {
		return errors.New("max armies per territory must be greater than zero")
	}
	if len(cfg.Territories) == 0 {
		return errors.New("no territories defined in configuration")
	}

	svg, err := os.Open(cfg.MapFile)
	if err != nil {
		return fmt.Errorf("failed to stat map file %s: %w", cfg.MapFile, err)
	}
	defer svg.Close()

	doc, err := xmlquery.Parse(svg)
	if err != nil {
		return fmt.Errorf("failed to parse map svg file %s: %w", cfg.MapFile, err)
	}
	for _, territory := range cfg.Territories {
		if xmlquery.FindOne(doc, fmt.Sprintf("//path[@id=%q]", territory.Abbreviation)) == nil {
			return fmt.Errorf("territory (path element with id of %q) not found in map file %s", territory.Name, cfg.MapFile)
		}
		if xmlquery.FindOne(doc, fmt.Sprintf("//circle[@id=%q]", territory.Abbreviation+"-armies")) == nil {
			return fmt.Errorf("army marker (circle element with id of %q) not found for territory %q in map file %s", territory.Abbreviation+"-armies", territory.Name, cfg.MapFile)
		}
	}

	return svg.Close()
}
