// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display

import (
	"errors"
	"reflect"

	"github.com/godbus/dbus/v5"
	sysdisplay "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.display1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

const MinScreenWidth = 1024.0;
const MinScreenHeight = 768.0;

var logger = log.NewLogger("daemon/display")

const (
	dbusServiceName = "org.deepin.dde.Display1"
	dbusInterface   = "org.deepin.dde.Display1"
	dbusPath        = "/org/deepin/dde/Display1"
)

var _dpy *Manager

var _greeterMode bool

func SetGreeterMode(val bool) {
	_greeterMode = val
}

type scaleFactorsHelper struct {
	changedCb func(factors map[string]float64) error
}

// ScaleFactorsHelper 全局的 scale factors 相关 helper，要传给 xsettings 模块。
var ScaleFactorsHelper scaleFactorsHelper

// 用于在 display.Start 还没被调用时，先由 xsettings.Start 调用了 ScaleFactorsHelper.SetScaleFactors, 缓存数据。
var _scaleFactors map[string]float64

func (h *scaleFactorsHelper) SetScaleFactors(factors map[string]float64) error {
	if _dpy == nil {
		_scaleFactors = factors
		return nil
	}
	return _dpy.setScaleFactors(factors)
}

func (h *scaleFactorsHelper) GetScaleFactors() (map[string]float64, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	sysDisplay := sysdisplay.NewDisplay(sysBus)
	cfgJson, err := sysDisplay.GetConfig(0)
	if err != nil {
		return nil, err
	}
	var rootCfg struct {
		Config struct {
			ScaleFactors map[string]float64
		}
	}
	err = jsonUnmarshal(cfgJson, &rootCfg)
	if err != nil {
		return nil, err
	}
	return rootCfg.Config.ScaleFactors, nil
}

func calcMaxScaleFactor(width, height float64) float64 {
	scaleList := []float64{ 1.0, 1.25, 1.5, 1.75, 2.0, 2.25, 2.5, 2.75, 3.0 }
    maxWScale := width / MinScreenWidth;
    maxHScale := height / MinScreenHeight;
	maxScale := 3.0
	if maxWScale < maxHScale {
		maxScale = maxWScale
	} else {
		maxScale = maxHScale
	}
	idx := 0
	for ; scaleList[idx] <= maxScale; idx++ {}
	return scaleList[idx-1]
}

func (h *scaleFactorsHelper) AdjustScaleFactor(width, height uint16) {
	// 每一次设置的模式的时候都需要注意，分辨率可能发生了变化，对于小分辨率的屏幕，不应该有过大的缩放比例
	// 此时应该自动调整为推荐的最大缩放比例
	scaleFactors, err := h.GetScaleFactors()
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Debug(scaleFactors)
	scaleFactor := scaleFactors["ALL"] // TODO 这里需要调整，不应该只考虑 ALL
	logger.Debug("Current scale factor: ", scaleFactor)
	maxScaleFactor := calcMaxScaleFactor(float64(width), float64(height))
	logger.Debug("Calculated max scale factor: ", maxScaleFactor)
	if scaleFactor > maxScaleFactor {
		scaleFactor = maxScaleFactor
	} else if scaleFactor < 1.0 {
		scaleFactor = 1.0
	}
	scaleFactors["ALL"] = scaleFactor
	logger.Debug(scaleFactors)
	err = h.SetScaleFactors(scaleFactors)
	if err != nil {
		logger.Warning(err)
		return
	}
}

func (h *scaleFactorsHelper) SetChangedCb(fn func(factors map[string]float64) error) {
	h.changedCb = fn
}

func (m *Manager) setScaleFactors(factors map[string]float64) error {
	logger.Debug("setScaleFactors", factors)
	m.sysConfig.mu.Lock()
	defer m.sysConfig.mu.Unlock()

	if reflect.DeepEqual(m.sysConfig.Config.ScaleFactors, factors) {
		return nil
	}
	m.sysConfig.Config.ScaleFactors = factors
	err := m.saveSysConfigNoLock("scale factors changed")
	if err != nil {
		logger.Warning(err)
	}
	return err
}

func Start(service *dbusutil.Service) error {
	m := newManager(service)
	m.init()

	if !_greeterMode {
		// 正常 startdde
		err := service.Export(dbusPath, m)
		if err != nil {
			return err
		}

		err = service.RequestName(dbusServiceName)
		if err != nil {
			return err
		}
	}
	_dpy = m
	return nil
}

func StartPart2() error {
	if _dpy == nil {
		return errors.New("_dpy is nil")
	}
	m := _dpy
	m.initSysDisplay()
	m.initTouchscreens()

	if !_greeterMode {
		controlRedshift("disable")
		m.applyColorTempConfig(m.DisplayMode)
	}

	return nil
}

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}
