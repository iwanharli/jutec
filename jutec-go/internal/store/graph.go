package store

import "context"

type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"` // site, ip, ns, tracking, artifact
	Meta  any    `json:"meta,omitempty"`
}

type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // shared_ip, shared_ns, shared_tracking, shared_artifact
}

type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GetNetworkGraph membangun graph dari data relasi di PostgreSQL.
// Hanya menyertakan site yang is_gambling=true untuk menjaga graph tetap relevan.
func (db *PostgresDB) GetNetworkGraph(ctx context.Context) (*GraphData, error) {
	nodes := map[string]GraphNode{}
	var edges []GraphEdge
	edgeSeen := map[string]bool{}

	addEdge := func(src, tgt, typ string) {
		key := src + "|" + tgt + "|" + typ
		if !edgeSeen[key] {
			edgeSeen[key] = true
			edges = append(edges, GraphEdge{Source: src, Target: tgt, Type: typ})
		}
	}

	// --- Nodes: gambling sites ---
	rows, err := db.Pool.Query(ctx, `
		SELECT id, url, confidence, priority FROM sites WHERE is_gambling = true
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var url, priority string
		var conf float64
		if err := rows.Scan(&id, &url, &conf, &priority); err != nil {
			continue
		}
		nid := nodeID("site", id)
		nodes[nid] = GraphNode{
			ID:    nid,
			Label: stripScheme(url),
			Type:  "site",
			Meta:  map[string]any{"url": url, "confidence": conf, "priority": priority},
		}
	}
	rows.Close()

	if len(nodes) == 0 {
		return &GraphData{Nodes: []GraphNode{}, Edges: []GraphEdge{}}, nil
	}

	// --- Edges: shared IP ---
	rows, err = db.Pool.Query(ctx, `
		SELECT i.id, i.address, s.site_id
		FROM ips i
		JOIN site_ips s ON s.ip_id = i.id
		JOIN sites st ON st.id = s.site_id AND st.is_gambling = true
	`)
	if err == nil {
		ipSites := map[int][]int{}
		ipAddr := map[int]string{}
		for rows.Next() {
			var ipID, siteID int
			var addr string
			if rows.Scan(&ipID, &addr, &siteID) == nil {
				ipSites[ipID] = append(ipSites[ipID], siteID)
				ipAddr[ipID] = addr
			}
		}
		rows.Close()
		for ipID, sites := range ipSites {
			if len(sites) < 2 {
				continue
			}
			nid := nodeID("ip", ipID)
			nodes[nid] = GraphNode{ID: nid, Label: ipAddr[ipID], Type: "ip"}
			for _, sid := range sites {
				sn := nodeID("site", sid)
				if _, ok := nodes[sn]; ok {
					addEdge(sn, nid, "shared_ip")
				}
			}
		}
	}

	// --- Edges: shared NS ---
	rows, err = db.Pool.Query(ctx, `
		SELECT n.id, n.host, sn.site_id
		FROM nameservers n
		JOIN site_ns sn ON sn.ns_id = n.id
		JOIN sites st ON st.id = sn.site_id AND st.is_gambling = true
	`)
	if err == nil {
		nsSites := map[int][]int{}
		nsHost := map[int]string{}
		for rows.Next() {
			var nsID, siteID int
			var host string
			if rows.Scan(&nsID, &host, &siteID) == nil {
				nsSites[nsID] = append(nsSites[nsID], siteID)
				nsHost[nsID] = host
			}
		}
		rows.Close()
		for nsID, sites := range nsSites {
			if len(sites) < 2 {
				continue
			}
			nid := nodeID("ns", nsID)
			nodes[nid] = GraphNode{ID: nid, Label: nsHost[nsID], Type: "ns"}
			for _, sid := range sites {
				sn := nodeID("site", sid)
				if _, ok := nodes[sn]; ok {
					addEdge(sn, nid, "shared_ns")
				}
			}
		}
	}

	// --- Edges: shared tracking ID ---
	rows, err = db.Pool.Query(ctx, `
		SELECT t.id, t.platform, t.value, st.site_id
		FROM tracking_ids t
		JOIN site_tracking st ON st.tracking_id = t.id
		JOIN sites s ON s.id = st.site_id AND s.is_gambling = true
	`)
	if err == nil {
		tSites := map[int][]int{}
		tLabel := map[int]string{}
		for rows.Next() {
			var tID, siteID int
			var platform, value string
			if rows.Scan(&tID, &platform, &value, &siteID) == nil {
				tSites[tID] = append(tSites[tID], siteID)
				tLabel[tID] = platform + ": " + value
			}
		}
		rows.Close()
		for tID, sites := range tSites {
			if len(sites) < 2 {
				continue
			}
			nid := nodeID("tracking", tID)
			nodes[nid] = GraphNode{ID: nid, Label: tLabel[tID], Type: "tracking"}
			for _, sid := range sites {
				sn := nodeID("site", sid)
				if _, ok := nodes[sn]; ok {
					addEdge(sn, nid, "shared_tracking")
				}
			}
		}
	}

	// --- Edges: shared artifact (Bank, E-Wallet, Crypto, Contact) ---
	rows, err = db.Pool.Query(ctx, `
		SELECT a.id, a.type, a.value, sa.site_id
		FROM artifacts a
		JOIN site_artifacts sa ON sa.artifact_id = a.id
		JOIN sites s ON s.id = sa.site_id AND s.is_gambling = true
		WHERE a.type IN ('Bank','E-Wallet','Crypto','Contact','VirtualAccount')
	`)
	if err == nil {
		aSites := map[int][]int{}
		aLabel := map[int]string{}
		for rows.Next() {
			var aID, siteID int
			var aType, value string
			if rows.Scan(&aID, &aType, &value, &siteID) == nil {
				aSites[aID] = append(aSites[aID], siteID)
				if len(value) > 20 {
					value = value[:20] + "…"
				}
				aLabel[aID] = aType + ": " + value
			}
		}
		rows.Close()
		for aID, sites := range aSites {
			if len(sites) < 2 {
				continue
			}
			nid := nodeID("artifact", aID)
			nodes[nid] = GraphNode{ID: nid, Label: aLabel[aID], Type: "artifact"}
			for _, sid := range sites {
				sn := nodeID("site", sid)
				if _, ok := nodes[sn]; ok {
					addEdge(sn, nid, "shared_artifact")
				}
			}
		}
	}

	// Collect only nodes that have at least one edge
	usedNodes := map[string]bool{}
	for _, e := range edges {
		usedNodes[e.Source] = true
		usedNodes[e.Target] = true
	}
	var nodeList []GraphNode
	for id, n := range nodes {
		if usedNodes[id] {
			nodeList = append(nodeList, n)
		}
	}

	return &GraphData{Nodes: nodeList, Edges: edges}, nil
}

func nodeID(typ string, id int) string {
	return typ + ":" + itoa(id)
}

func stripScheme(url string) string {
	for _, prefix := range []string{"https://www.", "http://www.", "https://", "http://"} {
		if len(url) > len(prefix) && url[:len(prefix)] == prefix {
			return url[len(prefix):]
		}
	}
	return url
}
