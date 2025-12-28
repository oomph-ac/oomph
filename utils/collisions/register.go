package collisions

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unsafe"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/oerror"

	_ "embed"
)

var (
	//go:embed data/blockCollisionShapes.json
	collisionShapeData []byte
	//go:embed data/blockStates.json
	blockStateData []byte

	// blockName -> blockState (check HashBlockProperties) -> bounding boxes
	collisionRegistry map[string]map[string][]cube.BBox
	// blockName -> bounding boxes
	staticCollisions map[string][]cube.BBox
)

func init() {
	type (
		encodedCollisionList        map[string][]int
		encodedCollisionDefinitions map[int][][6]float32 // [originX, originY, originZ, sizeX, sizeY, sizeZ]
		encodedBlockState           struct {
			Type  string `json:"type"`
			Value any    `json:"value"`
		}

		jsonCollisionList struct {
			Blocks encodedCollisionList        `json:"blocks"`
			Shapes encodedCollisionDefinitions `json:"shapes"`
		}
		jsonBlockData struct {
			Name    string                       `json:"name"`
			State   map[string]encodedBlockState `json:"states"`
			Version int64                        `json:"version"`
		}
	)

	encodedStateToMap := func(dat map[string]encodedBlockState) map[string]any {
		newDat := make(map[string]any, len(dat))
		for k, v := range dat {
			switch v.Type {
			case "string":
				newDat[k] = v.Value.(string)
			case "byte":
				newDat[k] = uint8(v.Value.(float64))
			case "int":
				newDat[k] = int32(v.Value.(float64))
			default:
				panic(oerror.New("unknown type in block state: %s", v.Type))
			}
		}
		return newDat
	}
	encodedCollisionToBBox := func(encodedBoxes [][6]float32) []cube.BBox {
		boxes := make([]cube.BBox, 0, len(encodedBoxes))
		for _, boxDat := range encodedBoxes {
			originX, originY, originZ := boxDat[0], boxDat[1], boxDat[2]
			halfSizeX, halfSizeY, halfSizeZ := boxDat[3]*0.5, boxDat[4]*0.5, boxDat[5]*0.5
			boxes = append(boxes, cube.Box(
				originX-halfSizeX, originY-halfSizeY, originZ-halfSizeZ,
				originX+halfSizeX, originY+halfSizeY, originZ+halfSizeZ,
			))
		}
		return boxes
	}

	var (
		collisionList  jsonCollisionList
		blockStateList []jsonBlockData

		blockStateCounter = make(map[string]int)
	)
	if err := json.Unmarshal(collisionShapeData, &collisionList); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(blockStateData, &blockStateList); err != nil {
		panic(err)
	}

	collisionRegistry = make(map[string]map[string][]cube.BBox)
	for _, blockData := range blockStateList {
		state := encodedStateToMap(blockData.State)
		stateHash := hashBlockProperties(state)
		currCount, ok := blockStateCounter[blockData.Name]
		if !ok {
			blockStateCounter[blockData.Name] = 0
		}
		coll := collisionList.Blocks[blockData.Name][currCount]
		prefixedName := "minecraft:" + blockData.Name
		blockReg, ok := collisionRegistry[prefixedName]
		if !ok {
			collisionRegistry[prefixedName] = make(map[string][]cube.BBox)
			blockReg = collisionRegistry[prefixedName]
		}
		blockReg[stateHash] = encodedCollisionToBBox(collisionList.Shapes[coll])
		blockStateCounter[blockData.Name]++
	}

	staticCollisions = make(map[string][]cube.BBox)
	// Iterate through all states to see if all are equal and therefore would not require us to hash block properties to find the correct bounding boxes.
check_loop:
	for blockName, states := range collisionRegistry {
		if len(states) == 0 {
			panic(oerror.New("block %s has no states for collision boxes", blockName))
		}
		// If a block only has one state, it means they have no states at all (hash="")
		if len(states) == 1 {
			staticCollisions[blockName] = states[""]
			continue
		}

		var lastSeenBBList []cube.BBox
		for _, bbList := range states {
			if lastSeenBBList == nil {
				lastSeenBBList = bbList
				continue
			}
			if !slices.Equal(lastSeenBBList, bbList) {
				continue check_loop
			}
		}
		staticCollisions[blockName] = lastSeenBBList
	}
}

func hashBlockProperties(properties map[string]any) string {
	if len(properties) == 0 {
		return ""
	}
	keys := make([]string, 0, len(properties))
	for k := range properties {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var b strings.Builder
	for _, k := range keys {
		switch v := properties[k].(type) {
		case bool:
			if v {
				b.WriteByte(1)
			} else {
				b.WriteByte(0)
			}
		case uint8:
			b.WriteByte(v)
		case int32:
			a := *(*[4]byte)(unsafe.Pointer(&v))
			b.Write(a[:])
		case string:
			b.WriteString(v)
		default:
			// If block encoding is broken, we want to find out as soon as possible. This saves a lot of time
			// debugging in-game.
			panic(fmt.Sprintf("invalid block property type %T for property %v", v, k))
		}
	}

	return b.String()
}
