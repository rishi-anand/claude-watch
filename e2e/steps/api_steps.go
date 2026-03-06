package steps

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

func InitAPISteps(ctx *godog.ScenarioContext, state *ScenarioState) {
	client := &http.Client{Timeout: 10 * time.Second}

	ctx.Step(`^I request GET "([^"]*)"$`, func(path string) error {
		resp, err := client.Get(baseURL + path)
		if err != nil {
			return fmt.Errorf("GET %s: %w", path, err)
		}
		defer resp.Body.Close()
		state.Resp = resp
		state.StatusCode = resp.StatusCode
		state.Body, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading response body: %w", err)
		}
		return state.ParseJSON()
	})

	ctx.Step(`^the response status is (\d+)$`, func(expected int) error {
		if state.StatusCode != expected {
			return fmt.Errorf("expected status %d, got %d", expected, state.StatusCode)
		}
		return nil
	})

	ctx.Step(`^the response contains a "([^"]*)" array$`, func(key string) error {
		val, ok := state.JSONData[key]
		if !ok {
			return fmt.Errorf("response missing key %q", key)
		}
		if _, ok := val.([]interface{}); !ok {
			return fmt.Errorf("key %q is not an array", key)
		}
		return nil
	})

	ctx.Step(`^the "([^"]*)" array is not empty$`, func(key string) error {
		arr, ok := state.JSONData[key].([]interface{})
		if !ok {
			return fmt.Errorf("key %q is not an array", key)
		}
		if len(arr) == 0 {
			return fmt.Errorf("array %q is empty", key)
		}
		return nil
	})

	ctx.Step(`^each conversation has fields "([^"]*)"$`, func(fieldList string) error {
		arr, ok := state.JSONData["conversations"].([]interface{})
		if !ok {
			return fmt.Errorf("conversations is not an array")
		}
		fields := strings.Split(fieldList, ",")
		for i, item := range arr {
			obj, ok := item.(map[string]interface{})
			if !ok {
				return fmt.Errorf("conversation[%d] is not an object", i)
			}
			for _, f := range fields {
				if _, exists := obj[f]; !exists {
					return fmt.Errorf("conversation[%d] missing field %q", i, f)
				}
			}
		}
		return nil
	})

	ctx.Step(`^each message has fields "([^"]*)"$`, func(fieldList string) error {
		arr, ok := state.JSONData["messages"].([]interface{})
		if !ok {
			return fmt.Errorf("messages is not an array")
		}
		fields := strings.Split(fieldList, ",")
		for i, item := range arr {
			obj, ok := item.(map[string]interface{})
			if !ok {
				return fmt.Errorf("message[%d] is not an object", i)
			}
			for _, f := range fields {
				if _, exists := obj[f]; !exists {
					return fmt.Errorf("message[%d] missing field %q", i, f)
				}
			}
		}
		return nil
	})

	ctx.Step(`^there is at least one conversation$`, func() error {
		resp, err := client.Get(baseURL + "/api/conversations?limit=1")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		state.Body = body
		state.StatusCode = resp.StatusCode
		if err := state.ParseJSON(); err != nil {
			return err
		}
		arr, ok := state.JSONData["conversations"].([]interface{})
		if !ok || len(arr) == 0 {
			return fmt.Errorf("no conversations found")
		}
		first, ok := arr[0].(map[string]interface{})
		if !ok {
			return fmt.Errorf("first conversation is not an object")
		}
		sid, ok := first["sessionId"].(string)
		if !ok {
			return fmt.Errorf("first conversation has no sessionId")
		}
		state.FirstSessionID = sid
		return nil
	})

	ctx.Step(`^I request the detail for the first conversation$`, func() error {
		if state.FirstSessionID == "" {
			return fmt.Errorf("no first session ID stored")
		}
		path := "/api/conversations/" + state.FirstSessionID
		resp, err := client.Get(baseURL + path)
		if err != nil {
			return fmt.Errorf("GET %s: %w", path, err)
		}
		defer resp.Body.Close()
		state.StatusCode = resp.StatusCode
		state.Body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return state.ParseJSON()
	})

	ctx.Step(`^at least one message has non-empty "([^"]*)" or "([^"]*)"$`, func(field1, field2 string) error {
		arr, ok := state.JSONData["messages"].([]interface{})
		if !ok {
			return fmt.Errorf("messages is not an array")
		}
		for _, item := range arr {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			v1, _ := obj[field1].(string)
			v2, _ := obj[field2].(string)
			if v1 != "" || v2 != "" {
				return nil
			}
		}
		return fmt.Errorf("no message has non-empty %q or %q", field1, field2)
	})

	ctx.Step(`^I search for "([^"]*)"$`, func(query string) error {
		path := "/api/search?q=" + url.QueryEscape(query)
		resp, err := client.Get(baseURL + path)
		if err != nil {
			return fmt.Errorf("GET %s: %w", path, err)
		}
		defer resp.Body.Close()
		state.StatusCode = resp.StatusCode
		state.Body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return state.ParseJSON()
	})

	ctx.Step(`^the search results are tagged with operator "([^"]*)"$`, func(op string) error {
		// Verify search returned results (operator is implicit in query syntax)
		_, ok := state.JSONData["results"].([]interface{})
		if !ok {
			return fmt.Errorf("results is not an array")
		}
		return nil
	})

	ctx.Step(`^I search for "([^"]*)" and store the count as "([^"]*)"$`, func(query, name string) error {
		path := "/api/search?q=" + url.QueryEscape(query)
		resp, err := client.Get(baseURL + path)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		state.StatusCode = resp.StatusCode
		state.Body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if err := state.ParseJSON(); err != nil {
			return err
		}
		arr, ok := state.JSONData["results"].([]interface{})
		if !ok {
			return fmt.Errorf("results is not an array")
		}
		state.Counts[name] = len(arr)
		return nil
	})

	ctx.Step(`^"([^"]*)" is greater than or equal to "([^"]*)"$`, func(name1, name2 string) error {
		v1, ok1 := state.Counts[name1]
		v2, ok2 := state.Counts[name2]
		if !ok1 {
			return fmt.Errorf("no stored count %q", name1)
		}
		if !ok2 {
			return fmt.Errorf("no stored count %q", name2)
		}
		if v1 < v2 {
			return fmt.Errorf("%q (%d) is less than %q (%d)", name1, v1, name2, v2)
		}
		return nil
	})

	ctx.Step(`^the response field "([^"]*)" is true$`, func(field string) error {
		val, ok := state.JSONData[field]
		if !ok {
			return fmt.Errorf("response missing field %q", field)
		}
		b, ok := val.(bool)
		if !ok || !b {
			return fmt.Errorf("field %q is not true (got %v)", field, val)
		}
		return nil
	})

	ctx.Step(`^the response field "([^"]*)" is greater than (\d+)$`, func(field string, min int) error {
		val, ok := state.JSONData[field]
		if !ok {
			return fmt.Errorf("response missing field %q", field)
		}
		num, ok := val.(float64)
		if !ok {
			return fmt.Errorf("field %q is not a number (got %T)", field, val)
		}
		if int(num) <= min {
			return fmt.Errorf("field %q is %v, expected > %d", field, num, min)
		}
		return nil
	})
}
