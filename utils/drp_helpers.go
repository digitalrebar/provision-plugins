package utils

import (
	"fmt"
	"strings"

	"github.com/VictorLowther/jsonpatch2"
	"github.com/digitalrebar/provision/v4/api"
	"github.com/digitalrebar/provision/v4/models"
)

var encodeJsonPtr = strings.NewReplacer("~", "~0", "/", "~1")

// String translates a pointerSegment into a regular string, encoding it as we go.
func makeJsonPtr(s string) string {
	return encodeJsonPtr.Replace(string(s))
}

/*
 * Helper function to lookup an object's parameter in aggregate so that
 * the parameter default could be retrieved if unset.
 */
func GetDrpParam(c *api.Client, objtype, objkey, param string) (interface{}, *models.Error) {
	var res interface{}
	err := c.Req().UrlFor(objtype, objkey, "params", param).Params("aggregate", "true").Do(&res)
	return res, ConvertError(400, err)
}

func AddDrpParam(c *api.Client, objtype, objkey, param string, new interface{}) *models.Error {
	var res interface{}
	base := map[string]interface{}{}
	if err := c.Req().UrlFor(objtype, objkey, "params").Do(&base); err != nil {
		return MakeError(400, fmt.Sprintf("Failed to get base parameter for %s: %v", param, err))
	}
	path := fmt.Sprintf("/%s", makeJsonPtr(param))
	patch := jsonpatch2.Patch{
		jsonpatch2.Operation{
			Op:    "test",
			Path:  "",
			Value: base,
		},
		jsonpatch2.Operation{
			Op:    "add",
			Path:  path,
			Value: new,
		},
	}
	if err := c.Req().Patch(patch).UrlFor(objtype, objkey, "params").Do(&res); err != nil {
		return MakeError(400, fmt.Sprintf("Failed to add parameter: %s: %v", param, err))
	}
	return nil
}

func RemoveDrpParam(c *api.Client, objtype, objkey, param string, old interface{}) *models.Error {
	var res interface{}
	if old == nil {
		var res interface{}
		err := c.Req().UrlFor(objtype, objkey, "params", param).Do(&res)
		if err != nil {
			return MakeError(400, fmt.Sprintf("Failed to get all parameters for %s: %v", objkey, err))
		}
		old = res
	}
	path := fmt.Sprintf("/%s", makeJsonPtr(param))
	patch := jsonpatch2.Patch{
		jsonpatch2.Operation{
			Op:    "test",
			Path:  path,
			Value: old,
		},
		jsonpatch2.Operation{
			Op:   "remove",
			Path: path,
		},
	}
	if err := c.Req().Patch(patch).UrlFor(objtype, objkey, "params").Do(&res); err != nil {
		return MakeError(400, fmt.Sprintf("Failed to remove parameter: %s: %v", param, err))
	}
	return nil
}

func SetDrpParam(c *api.Client, objtype, objkey, param string, old, new interface{}) *models.Error {
	var res interface{}
	path := fmt.Sprintf("/%s", makeJsonPtr(param))
	if old == nil {
		var res interface{}
		err := c.Req().UrlFor(objtype, objkey, "params", param).Do(&res)
		if err != nil {
			return MakeError(400, fmt.Sprintf("Failed to get all parameters for %s: %v", objkey, err))
		}
		old = res
	}
	patch := jsonpatch2.Patch{
		jsonpatch2.Operation{
			Op:    "test",
			Path:  path,
			Value: old,
		},
		jsonpatch2.Operation{
			Op:    "replace",
			Path:  path,
			Value: new,
		},
	}
	if err := c.Req().Patch(patch).UrlFor(objtype, objkey, "params").Do(&res); err != nil {
		return MakeError(400, fmt.Sprintf("Failed to set parameter: %s: %v", param, err))
	}
	return nil
}

func AddOrSetDrpParam(c *api.Client, objtype, objkey, param string, new interface{}) *models.Error {
	e := AddDrpParam(c, objtype, objkey, param, new)
	if e != nil {
		e2 := SetDrpParam(c, objtype, objkey, param, nil, new)
		if e2 == nil {
			return nil
		}
		e.AddError(e2)
	}
	return e
}

func GetDrpBooleanParam(c *api.Client, objtype, objkey, param string) (bool, *models.Error) {
	if v, err := GetDrpParam(c, objtype, objkey, param); err != nil {
		return false, err
	} else {
		if bval, err := ValidateBooleanValue(param, v); err != nil {
			return false, err
		} else {
			return bval, nil
		}
	}
}

func GetDrpStringParam(c *api.Client, objtype, objkey, param string) (string, *models.Error) {
	if v, err := GetDrpParam(c, objtype, objkey, param); err != nil {
		return "", err
	} else {
		if sval, err := ValidateStringValue(param, v); err != nil {
			return "", err
		} else {
			return sval, nil
		}
	}
}

func GetDrpIntParam(c *api.Client, objtype, objkey, param string) (int, *models.Error) {
	if v, err := GetDrpParam(c, objtype, objkey, param); err != nil {
		return 0, err
	} else {
		if ival, err := ValidateIntValue(param, v); err != nil {
			return 0, err
		} else {
			return ival, nil
		}
	}
}

func GetDrpObjByParam(c *api.Client, objtype, param, value string) (models.Model, *models.Error) {
	if arr, err := c.ListModel(objtype, param, fmt.Sprintf("Eq(%s)", value)); err != nil {
		return nil, ConvertError(400, err)
	} else {
		if len(arr) > 0 {
			return arr[0], nil
		}
	}
	return nil, nil
}

func GetDrpMachineByParam(c *api.Client, param, value string) (*models.Machine, *models.Error) {
	o, e := GetDrpObjByParam(c, "machines", param, value)
	if o == nil {
		return nil, e
	}
	return o.(*models.Machine), e
}

func DeleteDrpMachine(c *api.Client, uuid string) *models.Error {
	_, e := c.DeleteModel("machines", uuid)
	return ConvertError(400, e)
}

func AddToJobLog(c *api.Client, jid, data string) *models.Error {
	e := c.Req().Put([]byte(data)).UrlFor("jobs", jid, "log").Do(nil)
	return ConvertError(400, e)
}
