package display

import (
	"fmt"
	"os"
)

func (dpy *Manager) ListOutputNames() []string {
	return dpy.outputInfos.ListNames()
}

func (dpy *Manager) SetPrimary(primary string) error {
	err := dpy.doSetPrimary(primary, true)
	if err != nil {
		return err
	}
	return dpy.Save()
}

func (dpy *Manager) SwitchMode(mode uint8, name string) error {
	logger.Debug("[SwitchMode] will to mode:", mode, name)
	var err error
	switch mode {
	case DisplayModeMirror:
		err = dpy.switchToMirror()
	case DisplayModeExtend:
		err = dpy.switchToExtend()
	case DisplayModeOnlyOne:
		err = dpy.switchToOnlyOne(name)
	case DisplayModeCustom:
		err = dpy.switchToCustom()
	default:
		logger.Warning("Invalid display mode:", mode)
		return fmt.Errorf("Invalid display mode: %d", mode)
	}
	if err != nil {
		logger.Errorf("Switch mode to %d failed: %v", mode, err)
		return err
	}
	dpy.setPropDisplayMode(mode)
	return dpy.Save()
}

func (dpy *Manager) ApplyChanges() error {
	if !dpy.HasChanged {
		return nil
	}

	// err := dpy.doSetPrimary(dpy.Primary, true)
	// if err != nil {
	// 	logger.Error("Set primary failed:", dpy.Primary, err)
	// 	err = dpy.trySetPrimary(true)
	// 	if err != nil {
	// 		logger.Error("Try set primary failed:", err)
	// 		return err
	// 	}
	// }
	err := dpy.doApply(false)
	if err != nil {
		logger.Error("Apply changes failed:", err)
		return err
	}
	dpy.rotateInputPointor()
	return nil
}

func (dpy *Manager) ResetChanges() error {
	if !dpy.HasChanged {
		return nil
	}

	// firstly to find the matched config,
	// then update monitors from config, finaly apply it.
	id := dpy.Monitors.getMonitorsId()
	if len(id) == 0 {
		logger.Warning("No connected monitor found")
		return fmt.Errorf("No output connected")
	}

	cMonitor := dpy.config.get("id")
	if cMonitor == nil {
		logger.Warning("No config found for:", id)
		return fmt.Errorf("No config found for '%s'", id)
	}
	monitorsLocker.Lock()
	defer monitorsLocker.Unlock()
	for _, info := range cMonitor.BaseInfos {
		m := dpy.Monitors.getByName(info.Name)
		dpy.updateMonitorFromBaseInfo(m, info)
	}
	dpy.doSetPrimary(cMonitor.Primary, true)
	return dpy.doApply(false)
}

func (dpy *Manager) Save() error {
	if len(dpy.outputInfos) != 1 && dpy.DisplayMode != DisplayModeCustom {
		// if multi-output and not custom mode, nothing
		return nil
	}

	id := dpy.Monitors.getMonitorsId()
	if len(id) == 0 {
		logger.Warning("No output connected")
		return fmt.Errorf("No output connected")
	}
	cMonitor := configMonitor{
		Primary:   dpy.Primary,
		BaseInfos: dpy.Monitors.getBaseInfos(),
	}
	dpy.config.set(id, &cMonitor)

	return dpy.config.writeFile()
}

func (dpy *Manager) DeleteCustomConfig() error {
	id := dpy.Monitors.getMonitorsId()
	if !dpy.config.delete(id) {
		// no config found
		return nil
	}
	return dpy.config.writeFile()
}

func (dpy *Manager) Reset() error {
	// remove config file
	os.Remove(dpy.config.filename)
	return dpy.SwitchMode(DisplayModeExtend, "")
}
