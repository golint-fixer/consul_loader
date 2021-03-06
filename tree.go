package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	consul "github.com/hashicorp/consul/api"
)

// tree is a structure used to build a representation of the consul config.
type tree map[string]interface{}

// String returns a string representation of the Tree.
func (t tree) String() (repr string) {
	for k, v := range t {
		subTree, ok := v.(map[string]interface{})
		if ok {
			repr += fmt.Sprintf("%s: {%s},\n", k, tree(subTree).String())
		} else {
			repr += fmt.Sprintf("%s: %s\n", k, v)
		}
	}
	return
}

// add traverses the tree from the split key to find the proper place to put the value.
// A trie is built up from the keys for easy conversion to JSON.
func (t tree) add(k string, v interface{}) {
	// error if there is no key
	if k == "" {
		return
	}

	// split the key by segments to allow building a trie
	path := strings.Split(k, "/")

	if len(path) == 1 { // on the last key portion
		t[k] = v
	} else {
		subKey := path[0]
		subTree, exists := t[subKey]
		if !exists {
			t[subKey] = map[string]interface{}{}
			subTree = t[subKey] // make a reference to the subtree
		}

		// insert the value at the last brnach of the trie
		tree(subTree.(map[string]interface{})).add(strings.Join(path[1:], "/"), v)
	}
}

// build adds a series of KVPairs to the tree.
func (t tree) build(kvs consul.KVPairs, skip int) {
	for _, pair := range kvs {
		// use raw bytes if transferring from Consul key to Consul key
		t.add(pair.Key[skip:], string(pair.Value))
	}
}

// resolveBytes is a helper method to translate a byte array to
// a primitive type.
func resolveBytes(v interface{}) []byte {
	switch val := v.(type) {
	case []byte:
		return val
	case string:
		return []byte(val)
	case int64:
		return []byte(strconv.FormatInt(val, 10))
	case float64:
		return []byte(strconv.FormatInt(int64(val), 10))
	case int:
		return []byte(strconv.Itoa(val))
	default:
		log.Printf("interface: %#v", v)
		log.Fatal("Unsupported type, please file an issue")
	}

	return []byte{}
}

// update pushes a tree into a new provided key within Consul. The update is
// performed recursively through the tree.
func (t tree) update(base string) {
	for k, v := range t {
		key := base + "/" + k
		subMap, ok := v.(map[string]interface{})
		if ok {
			// update the sub tree
			tree(subMap).update(key)
			continue
		}

		// update the leaf
		push(key[1:], v)
	}
}

func push(key string, v interface{}) {
	val := resolveBytes(v)
	_, err := kv.Put(&consul.KVPair{
		Key:   key,
		Value: val,
	}, nil)
	if err != nil {
		log.Fatalf("Failed to write to Consul => {%s}", err)
	}
}
