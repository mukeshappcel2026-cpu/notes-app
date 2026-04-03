package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Memgraph uses the Bolt protocol and is Cypher-compatible.
// We use the Neo4j Go driver which works with both Memgraph and Neo4j,
// making future migration straightforward.

type Memgraph struct {
	driver neo4j.DriverWithContext
}

func NewMemgraph(host string, port int) (*Memgraph, error) {
	uri := fmt.Sprintf("bolt://%s:%d", host, port)
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.NoAuth())
	if err != nil {
		return nil, fmt.Errorf("memgraph driver: %w", err)
	}

	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("memgraph connectivity: %w", err)
	}

	mg := &Memgraph{driver: driver}
	if err := mg.initSchema(ctx); err != nil {
		return nil, fmt.Errorf("memgraph schema: %w", err)
	}
	return mg, nil
}

func (m *Memgraph) Close() {
	m.driver.Close(context.Background())
}

func (m *Memgraph) initSchema(ctx context.Context) error {
	session := m.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	constraints := []string{
		"CREATE INDEX ON :Agent(id)",
		"CREATE INDEX ON :Model(name)",
		"CREATE INDEX ON :Tool(name)",
	}

	for _, c := range constraints {
		_, err := session.Run(ctx, c, nil)
		if err != nil {
			// Memgraph may error on duplicate index creation; ignore
			continue
		}
	}
	return nil
}

// EnsureAgent creates or updates an Agent node.
func (m *Memgraph) EnsureAgent(ctx context.Context, agentID, team string) error {
	session := m.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MERGE (a:Agent {id: $id})
		SET a.team = $team, a.last_seen = timestamp()`,
		map[string]any{"id": agentID, "team": team})
	return err
}

// EnsureModel creates or updates a Model node and USES_MODEL edge.
func (m *Memgraph) EnsureModel(ctx context.Context, agentID, model string) error {
	session := m.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MERGE (a:Agent {id: $agent_id})
		MERGE (m:Model {name: $model})
		MERGE (a)-[r:USES_MODEL]->(m)
		SET r.last_seen = timestamp(), r.call_count = COALESCE(r.call_count, 0) + 1`,
		map[string]any{"agent_id": agentID, "model": model})
	return err
}

// EnsureTool creates or updates a Tool node and USES_TOOL edge.
func (m *Memgraph) EnsureTool(ctx context.Context, agentID, tool string) error {
	session := m.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MERGE (a:Agent {id: $agent_id})
		MERGE (t:Tool {name: $tool})
		MERGE (a)-[r:USES_TOOL]->(t)
		SET r.last_seen = timestamp(), r.call_count = COALESCE(r.call_count, 0) + 1`,
		map[string]any{"agent_id": agentID, "tool": tool})
	return err
}

// RecordAgentCall records an Agent→Agent communication edge.
func (m *Memgraph) RecordAgentCall(ctx context.Context, fromAgent, toAgent string) error {
	session := m.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	_, err := session.Run(ctx, `
		MERGE (from:Agent {id: $from})
		MERGE (to:Agent {id: $to})
		MERGE (from)-[r:CALLS]->(to)
		SET r.last_seen = timestamp(), r.call_count = COALESCE(r.call_count, 0) + 1`,
		map[string]any{"from": fromAgent, "to": toAgent})
	return err
}

// GetDependencyGraph returns all nodes and edges for visualization.
type GraphNode struct {
	ID    string `json:"id"`
	Type  string `json:"type"` // Agent, Model, Tool
	Label string `json:"label"`
	Team  string `json:"team,omitempty"`
}

type GraphEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Type      string `json:"type"` // USES_MODEL, USES_TOOL, CALLS
	CallCount int64  `json:"call_count"`
}

type DependencyGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

func (m *Memgraph) GetDependencyGraph(ctx context.Context) (*DependencyGraph, error) {
	session := m.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	g := &DependencyGraph{}

	// Get all nodes
	result, err := session.Run(ctx, `
		MATCH (n) WHERE n:Agent OR n:Model OR n:Tool
		RETURN labels(n)[0] AS type, n.id AS id, n.name AS name, n.team AS team`,
		nil)
	if err != nil {
		return nil, err
	}
	for result.Next(ctx) {
		record := result.Record()
		nodeType, _ := record.Get("type")
		id, _ := record.Get("id")
		name, _ := record.Get("name")
		team, _ := record.Get("team")

		label := ""
		nodeID := ""
		if id != nil {
			nodeID = id.(string)
			label = nodeID
		}
		if name != nil {
			label = name.(string)
			if nodeID == "" {
				nodeID = label
			}
		}

		teamStr := ""
		if team != nil {
			teamStr = team.(string)
		}

		g.Nodes = append(g.Nodes, GraphNode{
			ID:    nodeID,
			Type:  nodeType.(string),
			Label: label,
			Team:  teamStr,
		})
	}

	// Get all edges
	result, err = session.Run(ctx, `
		MATCH (a)-[r]->(b)
		RETURN a.id AS from_id, a.name AS from_name,
		       b.id AS to_id, b.name AS to_name,
		       type(r) AS rel_type, COALESCE(r.call_count, 0) AS call_count`,
		nil)
	if err != nil {
		return nil, err
	}
	for result.Next(ctx) {
		record := result.Record()
		fromID, _ := record.Get("from_id")
		fromName, _ := record.Get("from_name")
		toID, _ := record.Get("to_id")
		toName, _ := record.Get("to_name")
		relType, _ := record.Get("rel_type")
		callCount, _ := record.Get("call_count")

		from := coalesceStr(fromID, fromName)
		to := coalesceStr(toID, toName)

		g.Edges = append(g.Edges, GraphEdge{
			From:      from,
			To:        to,
			Type:      relType.(string),
			CallCount: callCount.(int64),
		})
	}

	return g, nil
}

// GetDownstream returns all agents downstream of the given agent.
func (m *Memgraph) GetDownstream(ctx context.Context, agentID string) ([]string, error) {
	session := m.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH (a:Agent {id: $id})-[:CALLS*1..5]->(downstream:Agent)
		RETURN DISTINCT downstream.id AS id`,
		map[string]any{"id": agentID})
	if err != nil {
		return nil, err
	}

	var agents []string
	for result.Next(ctx) {
		record := result.Record()
		id, _ := record.Get("id")
		if id != nil {
			agents = append(agents, id.(string))
		}
	}
	return agents, nil
}

// GetUpstream returns all agents that call the given agent.
func (m *Memgraph) GetUpstream(ctx context.Context, agentID string) ([]string, error) {
	session := m.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH (upstream:Agent)-[:CALLS*1..5]->(a:Agent {id: $id})
		RETURN DISTINCT upstream.id AS id`,
		map[string]any{"id": agentID})
	if err != nil {
		return nil, err
	}

	var agents []string
	for result.Next(ctx) {
		record := result.Record()
		id, _ := record.Get("id")
		if id != nil {
			agents = append(agents, id.(string))
		}
	}
	return agents, nil
}

func coalesceStr(vals ...any) string {
	for _, v := range vals {
		if v != nil {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
