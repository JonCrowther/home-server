package lifx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func LoadLights() ([]Light, error) {
	url := "https://api.lifx.com/v1/lights"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("LIFX_TOKEN"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, body)
	}

	var maps []map[string]any
	if err := json.Unmarshal(body, &maps); err != nil {
		return nil, err
	}

	lights := make([]Light, 0, len(maps))
	for _, m := range maps {
		lights = append(lights, ToLight(m))
	}
	return lights, nil
}

func SetLight(selector string, state LightState) error {
	url := "https://api.lifx.com/v1/lights/" + selector + "/state"

	body, err := json.Marshal(state)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("LIFX_TOKEN"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func SetMultipleLights(states ...SelectorState) error {
	url := "https://api.lifx.com/v1/lights/states"

	body, err := json.Marshal(map[string]any{"states": states})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("LIFX_TOKEN"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
