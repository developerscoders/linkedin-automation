package browser

import (
	"math/rand"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func (m *Manager) ApplyStealth(page *rod.Page) error {
	// 1. Disable navigator.webdriver
	_, err := page.EvalOnNewDocument(`() => {
        Object.defineProperty(navigator, 'webdriver', {
            get: () => undefined
        });
    }`)
	if err != nil {
		return err
	}

	// 2. Override navigator.plugins
	_, err = page.EvalOnNewDocument(`() => {
        Object.defineProperty(navigator, 'plugins', {
            get: () => [
                {
                    name: 'Chrome PDF Plugin',
                    filename: 'internal-pdf-viewer',
                    description: 'Portable Document Format'
                },
                {
                    name: 'Chrome PDF Viewer',
                    filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai',
                    description: 'Portable Document Format'
                },
                {
                    name: 'Native Client',
                    filename: 'internal-nacl-plugin',
                    description: 'Native Client Executable'
                }
            ]
        });
    }`)
	if err != nil {
		return err
	}

	// 3. Randomize canvas fingerprint
	_, err = page.EvalOnNewDocument(`() => {
        const originalGetContext = HTMLCanvasElement.prototype.getContext;
        HTMLCanvasElement.prototype.getContext = function(type, ...args) {
            const context = originalGetContext.apply(this, [type, ...args]);
            if (type === '2d') {
                const originalFillText = context.fillText;
                context.fillText = function(text, x, y, ...rest) {
                    // Add imperceptible noise
                    const noise = Math.random() * 0.0001;
                    return originalFillText.apply(this, [text, x + noise, y, ...rest]);
                };
            }
            return context;
        };
    }`)
	if err != nil {
		return err
	}

	// 4. Override WebGL parameters
	_, err = page.EvalOnNewDocument(`() => {
        const getParameter = WebGLRenderingContext.prototype.getParameter;
        WebGLRenderingContext.prototype.getParameter = function(parameter) {
            // UNMASKED_VENDOR_WEBGL
            if (parameter === 37445) {
                return 'Intel Inc.';
            }
            // UNMASKED_RENDERER_WEBGL
            if (parameter === 37446) {
                return 'Intel Iris OpenGL Engine';
            }
            return getParameter.call(this, parameter);
        };
    }`)
	if err != nil {
		return err
	}

	// 5. Set realistic navigator properties
	_, err = page.EvalOnNewDocument(`() => {
        Object.defineProperty(navigator, 'languages', {
            get: () => ['en-US', 'en']
        });
        Object.defineProperty(navigator, 'platform', {
            get: () => 'MacIntel'
        });
        Object.defineProperty(navigator, 'hardwareConcurrency', {
            get: () => 8
        });
        Object.defineProperty(navigator, 'deviceMemory', {
            get: () => 8
        });
    }`)
	if err != nil {
		return err
	}

	// 6. Override permissions
	_, err = page.EvalOnNewDocument(`() => {
        const originalQuery = window.navigator.permissions.query;
        window.navigator.permissions.query = (parameters) => (
            parameters.name === 'notifications' ?
                Promise.resolve({ state: Notification.permission }) :
                originalQuery(parameters)
        );
    }`)
	if err != nil {
		return err
	}

	// 7. Chrome runtime
	_, err = page.EvalOnNewDocument(`() => {
        window.chrome = {
            runtime: {}
        };
    }`)
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) RotateUserAgent() string {
	agents := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	}
	return agents[rand.Intn(len(agents))]
}

func (m *Manager) SetRandomViewport(page *rod.Page) error {
	viewports := []struct{ Width, Height int }{
		{1920, 1080},
		{1366, 768},
		{1536, 864},
		{1440, 900},
	}

	vp := viewports[rand.Intn(len(viewports))]
	return page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  vp.Width,
		Height: vp.Height,
		DeviceScaleFactor: 1,
		Mobile: false,
	})
}
