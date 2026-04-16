package prefill

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

const hostName = "com.mak.prefill"

type hostManifest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Path            string   `json:"path"`
	Type            string   `json:"type"`
	AllowedOrigins  []string `json:"allowed_origins"`
}

func newInstallCmd() *cobra.Command {
	var extensionID string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the mak prefill browser extension and native messaging host",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			extDir, err := extensionDir()
			if err != nil {
				return err
			}

			if err := writeExtensionFiles(extDir); err != nil {
				return fmt.Errorf("failed to write extension files: %w", err)
			}
			fmt.Printf("Extension files written to: %s\n", extDir)

			binPath, err := resolvedBinaryPath()
			if err != nil {
				return fmt.Errorf("could not resolve mak binary path: %w", err)
			}

			allowedOrigins := []string{}
			if extensionID != "" {
				allowedOrigins = []string{fmt.Sprintf("chrome-extension://%s/", extensionID)}
			}

			if err := installHostManifest(binPath, allowedOrigins); err != nil {
				return fmt.Errorf("failed to install native host manifest: %w", err)
			}

			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println()
			fmt.Println("  1. Open Chrome or Brave")
			fmt.Println("  2. Go to chrome://extensions (or brave://extensions)")
			fmt.Println("  3. Enable \"Developer mode\" (top right toggle)")
			fmt.Println("  4. Click \"Load unpacked\" and select:")
			fmt.Printf("       %s\n", extDir)
			fmt.Println("  5. Note the extension ID shown on the card")
			fmt.Println("  6. Run this command with your extension ID:")
			fmt.Printf("       mak prefill install --extension-id <YOUR_EXTENSION_ID>\n")
			fmt.Println()

			if extensionID != "" {
				fmt.Printf("Native host manifest updated with extension ID: %s\n", extensionID)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&extensionID, "extension-id", "", "Chrome extension ID to allow (run `mak prefill install` first, load the extension, then rerun with this flag)")
	return cmd
}

func extensionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "mak", "extension"), nil
}

func resolvedBinaryPath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Resolve symlinks so the manifest points to the real binary.
	return filepath.EvalSymlinks(path)
}

func installHostManifest(binPath string, allowedOrigins []string) error {
	manifest := hostManifest{
		Name:           hostName,
		Description:    "mak prefill native messaging host",
		Path:           binPath,
		Type:           "stdio",
		AllowedOrigins: allowedOrigins,
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	dirs := nativeHostDirs()
	installed := 0
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		dest := filepath.Join(dir, hostName+".json")
		if err := os.WriteFile(dest, data, 0644); err != nil {
			fmt.Printf("  Warning: could not write to %s: %v\n", dest, err)
			continue
		}
		fmt.Printf("  Installed host manifest: %s\n", dest)
		installed++
	}

	if installed == 0 {
		// Fallback: write to a known location and instruct user.
		fallback := filepath.Join(nativeHostFallbackDir(), hostName+".json")
		if err := os.MkdirAll(filepath.Dir(fallback), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fallback, data, 0644); err != nil {
			return err
		}
		fmt.Printf("  Host manifest written to: %s\n", fallback)
		fmt.Println("  (Chrome/Brave directories were not found — you may need to install them first)")
	}

	return nil
}

func nativeHostDirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts"),
			filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser", "NativeMessagingHosts"),
			filepath.Join(home, "Library", "Application Support", "Microsoft Edge", "NativeMessagingHosts"),
		}
	default: // linux
		return []string{
			filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts"),
			filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "NativeMessagingHosts"),
			filepath.Join(home, ".config", "microsoft-edge", "NativeMessagingHosts"),
		}
	}
}

func nativeHostFallbackDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "mak", "native-host")
}

// writeExtensionFiles generates all browser extension files into dir.
func writeExtensionFiles(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	files := map[string]string{
		"manifest.json": extensionManifest,
		"background.js": extensionBackground,
		"popup.html":    extensionPopupHTML,
		"popup.css":     extensionPopupCSS,
		"popup.js":      extensionPopupJS,
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}

	// Try to open the extension dir in Finder/Files after writing
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", dir).Start()
	case "linux":
		exec.Command("xdg-open", dir).Start()
	}

	return nil
}

// ── Extension file contents ───────────────────────────────────────────────────

const extensionManifest = `{
  "manifest_version": 3,
  "name": "mak prefill",
  "version": "1.0.0",
  "description": "Auto-fill login credentials managed by mak CLI",
  "permissions": [
    "nativeMessaging",
    "storage",
    "tabs",
    "scripting",
    "activeTab"
  ],
  "host_permissions": ["<all_urls>"],
  "background": {
    "service_worker": "background.js"
  },
  "action": {
    "default_popup": "popup.html",
    "default_title": "mak prefill"
  }
}
`

const extensionBackground = `// background.js — service worker
// Handles native messaging with the mak binary.

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.action === 'unlock') {
    unlockAndGetProjects(message.password)
      .then(result => sendResponse(result))
      .catch(err => sendResponse({ success: false, error: err.message }));
    return true; // keep channel open for async response
  }

  if (message.action === 'fill') {
    fillCredentials(message.tabId, message.email, message.password)
      .then(() => sendResponse({ success: true }))
      .catch(err => sendResponse({ success: false, error: err.message }));
    return true;
  }
});

async function unlockAndGetProjects(password) {
  return new Promise((resolve, reject) => {
    let port;
    try {
      port = chrome.runtime.connectNative('com.mak.prefill');
    } catch (e) {
      reject(new Error('Could not connect to mak. Make sure mak is installed and you have run: mak prefill install --extension-id <id>'));
      return;
    }

    const timeout = setTimeout(() => {
      port.disconnect();
      reject(new Error('Native host timed out.'));
    }, 10000);

    port.onMessage.addListener(msg => {
      clearTimeout(timeout);
      port.disconnect();
      resolve(msg);
    });

    port.onDisconnect.addListener(() => {
      clearTimeout(timeout);
      const err = chrome.runtime.lastError;
      if (err) reject(new Error(err.message));
    });

    port.postMessage({ action: 'unlock', password });
  });
}

async function fillCredentials(tabId, email, password) {
  await chrome.scripting.executeScript({
    target: { tabId },
    func: (email, password) => {
      const emailSelectors = [
        'input[type="email"]',
        'input[name*="email" i]',
        'input[name*="user" i]',
        'input[name*="login" i]',
        'input[id*="email" i]',
        'input[id*="user" i]',
        'input[placeholder*="email" i]',
        'input[placeholder*="username" i]',
      ];

      let emailInput = null;
      for (const sel of emailSelectors) {
        emailInput = document.querySelector(sel);
        if (emailInput) break;
      }

      const passwordInput = document.querySelector('input[type="password"]');

      function fill(input, value) {
        if (!input) return false;
        const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
          window.HTMLInputElement.prototype, 'value'
        ).set;
        nativeInputValueSetter.call(input, value);
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
        return true;
      }

      const emailFilled = fill(emailInput, email);
      const passwordFilled = fill(passwordInput, password);
      return { emailFilled, passwordFilled };
    },
    args: [email, password],
  });
}
`

const extensionPopupHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="popup.css">
</head>
<body>
  <div id="app">
    <div id="lock-screen" class="hidden">
      <div class="lock-header">
        <div class="logo">mak prefill</div>
        <div class="lock-subtitle">Enter your master password</div>
      </div>
      <div class="lock-form">
        <input id="master-password" type="password" placeholder="Master password" autocomplete="current-password">
        <button id="unlock-btn">Unlock</button>
        <div id="lock-error" class="error hidden"></div>
      </div>
    </div>

    <div id="project-list" class="hidden">
      <div id="projects-container"></div>
      <button id="lock-btn" class="lock-btn">Lock</button>
    </div>

    <div id="fill-toast" class="toast hidden"></div>
  </div>
  <script src="popup.js"></script>
</body>
</html>
`

const extensionPopupCSS = `* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  width: 340px;
  min-height: 200px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-size: 14px;
  background: #f0f0f0;
  color: #1a1a1a;
}

#app {
  padding: 12px;
}

.hidden { display: none !important; }

/* Lock screen */
.lock-header {
  text-align: center;
  padding: 20px 0 16px;
}
.logo {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.3px;
  margin-bottom: 4px;
}
.lock-subtitle {
  font-size: 12px;
  color: #666;
}
.lock-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-bottom: 8px;
}
.lock-form input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #ddd;
  border-radius: 8px;
  font-size: 14px;
  outline: none;
  background: #fff;
}
.lock-form input:focus {
  border-color: #555;
}
.lock-form button {
  width: 100%;
  padding: 10px;
  background: #1a1a1a;
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
}
.lock-form button:hover { background: #333; }
.lock-form button:disabled { background: #999; cursor: not-allowed; }

/* Error */
.error {
  font-size: 12px;
  color: #c00;
  text-align: center;
  padding: 4px 0;
}

/* Project list */
.project-section {
  margin-bottom: 14px;
}
.project-name {
  font-size: 12px;
  font-weight: 600;
  color: #555;
  text-transform: uppercase;
  letter-spacing: 0.4px;
  margin-bottom: 6px;
  padding: 0 4px;
}
.domain-card {
  background: #fff;
  border-radius: 10px;
  overflow: hidden;
}
.domain-row {
  display: flex;
  align-items: center;
  padding: 12px 14px;
  cursor: pointer;
  transition: background 0.1s;
  border-bottom: 1px solid #f0f0f0;
}
.domain-row:last-child { border-bottom: none; }
.domain-row:hover { background: #f8f8f8; }
.domain-row.current-page { background: #f0faf0; }
.domain-row.current-page:hover { background: #e8f5e8; }
.domain-label {
  flex: 1;
  font-size: 14px;
  font-weight: 400;
}
.domain-label small {
  display: block;
  font-size: 11px;
  color: #999;
  margin-top: 1px;
}
.domain-action {
  display: flex;
  align-items: center;
  gap: 6px;
}
.btn-fill {
  padding: 4px 10px;
  background: #1a1a1a;
  color: #fff;
  border: none;
  border-radius: 6px;
  font-size: 12px;
  cursor: pointer;
}
.btn-fill:hover { background: #333; }
.arrow {
  font-size: 16px;
  color: #999;
}

/* Lock button */
.lock-btn {
  width: 100%;
  margin-top: 4px;
  padding: 8px;
  background: transparent;
  color: #999;
  border: 1px solid #ddd;
  border-radius: 8px;
  font-size: 12px;
  cursor: pointer;
}
.lock-btn:hover { background: #f0f0f0; color: #666; }

/* Toast */
.toast {
  position: fixed;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  background: #1a1a1a;
  color: #fff;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 13px;
  white-space: nowrap;
}
`

const extensionPopupJS = `// popup.js
const SESSION_KEY = 'mak_prefill_projects';

let currentTabUrl = '';

document.addEventListener('DOMContentLoaded', async () => {
  // Get current tab URL
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  currentTabUrl = tab?.url ?? '';

  // Check if projects are already unlocked in session storage
  const result = await chrome.storage.session.get(SESSION_KEY);
  if (result[SESSION_KEY]) {
    showProjects(result[SESSION_KEY]);
  } else {
    showLockScreen();
  }

  // Unlock button
  document.getElementById('unlock-btn').addEventListener('click', handleUnlock);
  document.getElementById('master-password').addEventListener('keydown', e => {
    if (e.key === 'Enter') handleUnlock();
  });

  // Lock button
  document.getElementById('lock-btn').addEventListener('click', async () => {
    await chrome.storage.session.remove(SESSION_KEY);
    showLockScreen();
  });
});

function showLockScreen() {
  document.getElementById('lock-screen').classList.remove('hidden');
  document.getElementById('project-list').classList.add('hidden');
  document.getElementById('master-password').value = '';
  document.getElementById('lock-error').classList.add('hidden');
  setTimeout(() => document.getElementById('master-password').focus(), 50);
}

function showProjects(projects) {
  document.getElementById('lock-screen').classList.add('hidden');
  document.getElementById('project-list').classList.remove('hidden');
  renderProjects(projects);
}

async function handleUnlock() {
  const btn = document.getElementById('unlock-btn');
  const errEl = document.getElementById('lock-error');
  const pw = document.getElementById('master-password').value;

  if (!pw) return;

  btn.disabled = true;
  btn.textContent = 'Unlocking...';
  errEl.classList.add('hidden');

  const response = await chrome.runtime.sendMessage({ action: 'unlock', password: pw });

  btn.disabled = false;
  btn.textContent = 'Unlock';

  if (!response || !response.success) {
    errEl.textContent = response?.error ?? 'Failed to connect to mak.';
    errEl.classList.remove('hidden');
    return;
  }

  const projects = response.projects ?? [];
  await chrome.storage.session.set({ [SESSION_KEY]: projects });
  showProjects(projects);
}

function renderProjects(projects) {
  const container = document.getElementById('projects-container');
  container.innerHTML = '';

  if (!projects.length) {
    container.innerHTML = '<div style="text-align:center;color:#999;padding:20px 0;">No projects found.<br>Run <code>mak prefill add</code> to add one.</div>';
    return;
  }

  for (const project of projects) {
    const section = document.createElement('div');
    section.className = 'project-section';

    const nameEl = document.createElement('div');
    nameEl.className = 'project-name';
    nameEl.textContent = project.name;
    section.appendChild(nameEl);

    const card = document.createElement('div');
    card.className = 'domain-card';

    for (const domain of project.domains) {
      const isCurrentPage = isUrlMatch(currentTabUrl, domain.url);
      const row = document.createElement('div');
      row.className = 'domain-row' + (isCurrentPage ? ' current-page' : '');

      const labelEl = document.createElement('div');
      labelEl.className = 'domain-label';
      labelEl.textContent = domain.label;
      const urlSmall = document.createElement('small');
      urlSmall.textContent = truncateUrl(domain.url);
      labelEl.appendChild(urlSmall);

      const actionEl = document.createElement('div');
      actionEl.className = 'domain-action';

      if (isCurrentPage) {
        const fillBtn = document.createElement('button');
        fillBtn.className = 'btn-fill';
        fillBtn.textContent = 'Fill';
        fillBtn.addEventListener('click', async (e) => {
          e.stopPropagation();
          await handleFill(domain);
        });
        actionEl.appendChild(fillBtn);
      } else {
        const arrow = document.createElement('span');
        arrow.className = 'arrow';
        arrow.textContent = '→';
        actionEl.appendChild(arrow);

        row.addEventListener('click', () => handleNavigate(domain));
      }

      row.appendChild(labelEl);
      row.appendChild(actionEl);
      card.appendChild(row);
    }

    section.appendChild(card);
    container.appendChild(section);
  }
}

async function handleFill(domain) {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab) return;

  const result = await chrome.runtime.sendMessage({
    action: 'fill',
    tabId: tab.id,
    email: domain.email,
    password: domain.password,
  });

  if (result?.success) {
    showToast('Form filled!');
  } else {
    showToast('Could not fill — no login form found?');
  }
}

async function handleNavigate(domain) {
  const isLocalhost = domain.url.includes('localhost') || domain.url.includes('127.0.0.1');

  // Store pending fill so the extension can fill after navigation
  await chrome.storage.session.set({
    mak_pending_fill: { email: domain.email, password: domain.password, url: domain.url }
  });

  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tab) {
    await chrome.tabs.update(tab.id, { url: domain.url });
  }
  window.close();
}

function isUrlMatch(tabUrl, domainUrl) {
  if (!tabUrl || !domainUrl) return false;
  try {
    const tab = new URL(tabUrl);
    const dom = new URL(domainUrl);
    return tab.hostname === dom.hostname && tab.pathname.startsWith(dom.pathname);
  } catch {
    return false;
  }
}

function truncateUrl(url) {
  try {
    const u = new URL(url);
    return u.hostname + (u.pathname !== '/' ? u.pathname : '');
  } catch {
    return url;
  }
}

function showToast(msg) {
  const toast = document.getElementById('fill-toast');
  toast.textContent = msg;
  toast.classList.remove('hidden');
  setTimeout(() => toast.classList.add('hidden'), 2000);
}
`
