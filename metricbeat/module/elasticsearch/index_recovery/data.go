// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package index_recovery

import (
	"encoding/json"
	"errors"

	"fmt"

	s "github.com/elastic/beats/v7/libbeat/common/schema"
	c "github.com/elastic/beats/v7/libbeat/common/schema/mapstriface"
	"github.com/elastic/beats/v7/metricbeat/helper/elastic"
	"github.com/elastic/beats/v7/metricbeat/mb"
	"github.com/elastic/beats/v7/metricbeat/module/elasticsearch"
	"github.com/elastic/elastic-agent-libs/mapstr"
)

var (
	schema = s.Schema{
		// This is all shard information and should be linked to elasticsearch.shard.*
		// as soon as field aliases are available.
		"id":      c.Int("id", s.Optional),
		"type":    c.Str("type", s.Optional),
		"primary": c.Bool("primary", s.Optional),
		"stage":   c.Str("stage", s.Optional),

		// As soon as we have field alias feature available, source and target should
		// link to elasticsearch.node.* as it's not specific information.
		"source": c.Dict("source", s.Schema{
			"id":                c.Str("id", s.Optional),
			"host":              c.Str("host", s.Optional),
			"name":              c.Str("name", s.Optional),
			"transport_address": c.Str("transport_address", s.Optional),
		}),
		"target": c.Dict("target", s.Schema{
			"id":                c.Str("id", s.Optional),
			"host":              c.Str("host", s.Optional),
			"name":              c.Str("name", s.Optional),
			"transport_address": c.Str("transport_address", s.Optional),
		}),
		"index": s.Object{
			"files": c.Dict("index.files", s.Schema{
				"percent":   c.Str("percent", s.Optional),
				"reused":    c.Int("reused", s.Optional),
				"recovered": c.Int("recovered", s.Optional),
				"total":     c.Int("total", s.Optional),
			}),
			"size": c.Dict("index.size", s.Schema{
				"recovered_in_bytes": c.Int("recovered_in_bytes", s.Optional),
				"reused_in_bytes":    c.Int("reused_in_bytes", s.Optional),
				"total_in_bytes":     c.Int("total_in_bytes", s.Optional),
			}),
		},
		"translog": c.Dict("translog", s.Schema{
			"total":          c.Int("total", s.Optional),
			"percent":        c.Str("percent", s.Optional),
			"total_on_start": c.Int("total_on_start", s.Optional),
		}),

		"stop_time": s.Object{
			"ms": c.Int("stop_time_in_millis", s.Optional),
		},

		"start_time": s.Object{
			"ms": c.Int("start_time_in_millis", s.Optional),
		},

		"total_time": s.Object{
			"ms": c.Int("total_time_in_millis", s.Optional),
		},
	}
)

func eventsMapping(r mb.ReporterV2, info elasticsearch.Info, content []byte, isXpack bool) error {

	var data map[string]map[string][]map[string]interface{}

	err := json.Unmarshal(content, &data)
	if err != nil {
		return fmt.Errorf("failure parsing Elasticsearch Recovery API response: %w", err)
	}

	var errs []error
	for indexName, d := range data {
		shards, ok := d["shards"]
		if !ok {
			errs = append(errs, elastic.MakeErrorForMissingField(indexName+".shards", elastic.Elasticsearch))
			continue
		}
		for _, data := range shards {
			event := mb.Event{}

			event.RootFields = mapstr.M{}
			_, _ = event.RootFields.Put("service.name", elasticsearch.ModuleName)

			event.ModuleFields = mapstr.M{}
			_, _ = event.ModuleFields.Put("cluster.name", info.ClusterName)
			_, _ = event.ModuleFields.Put("cluster.id", info.ClusterID)
			_, _ = event.ModuleFields.Put("index.name", indexName)

			event.MetricSetFields, err = schema.Apply(data)
			if err != nil {
				errs = append(errs, fmt.Errorf("failure applying shard schema: %w", err))

				continue
			}
			_, _ = event.MetricSetFields.Put("name", indexName)

			// xpack.enabled in config using standalone metricbeat writes to `.monitoring` instead of `metricbeat-*`
			// When using Agent, the index name is overwritten anyways.
			if isXpack {
				index := elastic.MakeXPackMonitoringIndexName(elastic.Elasticsearch)
				event.Index = index
			}

			r.Event(event)
		}
	}
	return errors.Join(errs...)
}
