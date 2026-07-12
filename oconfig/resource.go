package oconfig

type ResourceOpts struct {
	ResourceFolder string `json:"resource_folder" comment:"The folder where resource packs are stored. If your resource pack requires a content key, list them in a JSON file with a map of the pack UUID to the content key."`
	RequirePacks   bool   `json:"require_packs" comment:"Set to true if you require players to download the resource packs stored on the proxy."`
}

func Resource() ResourceOpts {
	return Global.Resource
}
