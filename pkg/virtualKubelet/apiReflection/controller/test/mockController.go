package test

import (
	"errors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

type MockController struct {
	cache      map[string]interface{}
	cacheLocal map[string]interface{}
}

func (c *MockController) GetMirroredObjectByKey(api apiReflection.ApiType, namespace string, name string) interface{} {
	if c.cacheLocal == nil {
		c.cacheLocal = map[string]interface{}{}
	}
	obj, ok := c.cacheLocal[name]
	if !ok {
		return nil
	}
	return obj
}

func (c *MockController) GetMirroringObjectByKey(api apiReflection.ApiType, namespace string, name string) (interface{}, error) {
	if c.cache == nil {
		c.cache = map[string]interface{}{}
	}
	obj, ok := c.cache[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return obj, nil
}

func (c *MockController) ListMirroringObjects(api apiReflection.ApiType, namespace string) ([]interface{}, error) {
	res := []interface{}{}
	for _, v := range c.cache {
		res = append(res, v)
	}
	return res, nil
}

func (c *MockController) ListMirroredObjects(api apiReflection.ApiType, namespace string) []interface{} {
	res := []interface{}{}
	for _, v := range c.cacheLocal {
		res = append(res, v)
	}
	return res
}

func (c *MockController) AddMirroredObject(object interface{}, name string) {
	if c.cacheLocal == nil {
		c.cacheLocal = map[string]interface{}{}
	}
	c.cacheLocal[name] = object
}

func (c *MockController) AddMirroringObject(object interface{}, name string) {
	if c.cache == nil {
		c.cache = map[string]interface{}{}
	}
	c.cache[name] = object
}
