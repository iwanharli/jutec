package store

import (
	"context"
	"fmt"
	"log"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Neo4jDB struct {
	Driver neo4j.DriverWithContext
}

func NewNeo4jConnection(uri, username, password string) (*Neo4jDB, error) {
	ctx := context.Background()
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %v", err)
	}

	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("failed to verify neo4j connectivity: %v", err)
	}

	log.Println("Successfully connected to Neo4j")
	return &Neo4jDB{Driver: driver}, nil
}

func (db *Neo4jDB) Close(ctx context.Context) error {
	return db.Driver.Close(ctx)
}

func (db *Neo4jDB) SaveAnalysisResult(ctx context.Context, url string, isGambling bool, confidence float64, priority string, keywords []string, htmlHash string, scriptsHash string, artifacts []map[string]string) error {
	session := db.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, `
			MERGE (s:Site {url: $url})
			SET s.is_gambling = $is_gambling,
			    s.confidence  = $confidence,
			    s.priority    = $priority,
			    s.updated_at  = datetime()
		`, map[string]interface{}{
			"url":         url,
			"is_gambling": isGambling,
			"confidence":  confidence,
			"priority":    priority,
		})
		if err != nil {
			return nil, err
		}

		for _, kw := range keywords {
			if _, err := tx.Run(ctx, `
				MATCH (s:Site {url: $url})
				MERGE (k:Keyword {name: $kw})
				MERGE (s)-[:CONTAINS_KEYWORD]->(k)
			`, map[string]interface{}{"url": url, "kw": kw}); err != nil {
				return nil, err
			}
		}

		if htmlHash != "" {
			if _, err := tx.Run(ctx, `
				MATCH (s:Site {url: $url})
				MERGE (f:Fingerprint {html_hash: $html_hash})
				SET f.scripts_hash = $scripts_hash
				MERGE (s)-[:HAS_FINGERPRINT]->(f)
			`, map[string]interface{}{
				"url":          url,
				"html_hash":    htmlHash,
				"scripts_hash": scriptsHash,
			}); err != nil {
				return nil, err
			}
		}

		for _, art := range artifacts {
			if _, err := tx.Run(ctx, `
				MATCH (s:Site {url: $url})
				MERGE (a:Artifact {value: $value})
				SET a.type = $type, a.name = $name
				MERGE (s)-[:USES_PAYMENT]->(a)
			`, map[string]interface{}{
				"url":   url,
				"value": art["value"],
				"type":  art["type"],
				"name":  art["name"],
			}); err != nil {
				return nil, err
			}
		}

		return nil, nil
	})

	return err
}

func (db *Neo4jDB) SaveInfraInfo(ctx context.Context, url string, ips []string, nss []string, sslThumb string) error {
	session := db.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		for _, ip := range ips {
			if _, err := tx.Run(ctx, `
				MATCH (s:Site {url: $url})
				MERGE (i:IP {address: $ip})
				MERGE (s)-[:RESOLVES_TO]->(i)
			`, map[string]interface{}{"url": url, "ip": ip}); err != nil {
				return nil, err
			}
		}

		for _, ns := range nss {
			if _, err := tx.Run(ctx, `
				MATCH (s:Site {url: $url})
				MERGE (n:NS {host: $ns})
				MERGE (s)-[:USES_NS]->(n)
			`, map[string]interface{}{"url": url, "ns": ns}); err != nil {
				return nil, err
			}
		}

		if sslThumb != "" {
			if _, err := tx.Run(ctx, `
				MATCH (s:Site {url: $url})
				MERGE (c:Certificate {thumbprint: $thumb})
				MERGE (s)-[:HAS_CERT]->(c)
			`, map[string]interface{}{"url": url, "thumb": sslThumb}); err != nil {
				return nil, err
			}
		}

		return nil, nil
	})

	return err
}

func (db *Neo4jDB) GetConnectedSites(ctx context.Context, attrType, attrValue string) ([]map[string]interface{}, error) {
	session := db.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		var query string
		switch attrType {
		case "Contact", "Bank", "QRIS", "Crypto", "E-Wallet":
			query = `MATCH (a:Artifact {value: $value})<-[:USES_PAYMENT]-(s:Site) RETURN s.url as url, s.priority as priority, s.confidence as confidence`
		case "IP":
			query = `MATCH (i:IP {address: $value})<-[:RESOLVES_TO]-(s:Site) RETURN s.url as url, s.priority as priority, s.confidence as confidence`
		case "NS":
			query = `MATCH (n:NS {host: $value})<-[:USES_NS]-(s:Site) RETURN s.url as url, s.priority as priority, s.confidence as confidence`
		case "SSL":
			query = `MATCH (c:Certificate {thumbprint: $value})<-[:HAS_CERT]-(s:Site) RETURN s.url as url, s.priority as priority, s.confidence as confidence`
		case "Fingerprint":
			query = `MATCH (f:Fingerprint {html_hash: $value})<-[:HAS_FINGERPRINT]-(s:Site) RETURN s.url as url, s.priority as priority, s.confidence as confidence`
		default:
			return nil, fmt.Errorf("unsupported attribute type: %s", attrType)
		}

		res, err := tx.Run(ctx, query, map[string]interface{}{"value": attrValue})
		if err != nil {
			return nil, err
		}

		var sites []map[string]interface{}
		for res.Next(ctx) {
			record := res.Record()
			url, _ := record.Get("url")
			priority, _ := record.Get("priority")
			confidence, _ := record.Get("confidence")
			sites = append(sites, map[string]interface{}{
				"url":        url,
				"priority":   priority,
				"confidence": confidence,
			})
		}
		return sites, nil
	})

	if err != nil {
		return nil, err
	}
	if result == nil {
		return []map[string]interface{}{}, nil
	}
	return result.([]map[string]interface{}), nil
}
