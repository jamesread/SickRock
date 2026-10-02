import assert from 'node:assert'
import { Builder, By, Key, until } from 'selenium-webdriver'
import chrome from 'selenium-webdriver/chrome.js'

const base = process.env.SICKROCK_BASE_URL || 'http://127.0.0.1:18080'

describe('SickRock smoke', function () {
  this.timeout(30000)
  let driver

  before(async function () {
    const options = new chrome.Options()
    options.addArguments('--headless=new', '--no-sandbox', '--disable-dev-shm-usage')
    driver = await new Builder().forBrowser('chrome').setChromeOptions(options).build()
  })

  after(async function () {
    if (driver) {
      await driver.quit()
    }
  })

  it('serves the login page with SickRock title', async function () {
    await driver.get(base + '/login')
    await driver.wait(until.elementLocated(By.css('body')), 10000)
    const title = await driver.getTitle()
    assert.match(title, /SickRock/)
    const heading = await driver.findElement(By.css('h2')).getText()
    assert.match(heading, /SickRock/)
  })

  it('redirects unauthenticated users from home to login', async function () {
    await driver.get(base + '/')
    await driver.wait(until.elementLocated(By.css('#username')), 10000)
    const currentUrl = await driver.getCurrentUrl()
    assert.match(currentUrl, /\/login/)
  })

  it('logs in with the default admin user and reaches home', async function () {
    await driver.get(base + '/login')
    await driver.wait(until.elementLocated(By.css('#username')), 10000)
    await driver.findElement(By.css('#username')).sendKeys('admin')
    await driver.findElement(By.css('#password')).sendKeys('admin')
    await driver.findElement(By.css('form.local-login-form button[type="submit"]')).click()
    await driver.wait(until.urlMatches(/\/($|\?|#)/), 15000)
    await driver.wait(until.elementLocated(By.css('body')), 10000)
    const body = await driver.findElement(By.css('body')).getText()
    assert.match(body, /Welcome|Recently Viewed|SickRock/)
  })

  it('renders and filters a configured table with PicoCrank', async function () {
    const authenticated = await driver.executeAsyncScript(`
      const done = arguments[arguments.length - 1]
      const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
      ;(async () => {
        const loginResponse = await fetch('/api/sickrock.SickRock/Login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username: 'admin', password: 'admin' }),
        })
        const login = await loginResponse.json()
        if (!login.success || !login.token) {
          done(false)
          return
        }

        for (let attempt = 0; attempt < 10; attempt += 1) {
          const response = await fetch('/api/sickrock.SickRock/ValidateToken', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer ' + login.token,
              'Session-Token': login.token,
            },
            body: JSON.stringify({ token: login.token }),
          })
          const validation = await response.json()
          if (validation.valid) {
            localStorage.setItem('session-token', login.token)
            done(true)
            return
          }
          await pause(100)
        }
        done(false)
      })().catch(() => done(false))
    `)
    assert.equal(authenticated, true)

    await driver.get(base + '/table/table_settings')
    const table = await driver.wait(until.elementLocated(By.css('table.row-hover')), 15000)
    await driver.wait(async () => (await table.findElements(By.css('tbody tr'))).length > 0, 10000)

    await table.findElement(By.css('button[aria-label="Row actions"]')).click()
    const rowMenu = await driver.wait(
      until.elementLocated(By.css('.table-row-context-menu[role="menu"]')),
      5000,
    )
    assert.match(await rowMenu.getText(), /View[\s\S]*Delete/)
    await driver.actions().sendKeys(Key.ESCAPE).perform()
    await driver.wait(until.stalenessOf(rowMenu), 5000)

    const settingKeyHeader = await table.findElement(By.xpath('.//th[contains(., "setting_key")]'))
    await driver.actions().contextClick(settingKeyHeader).perform()

    const filterPopover = await driver.wait(
      until.elementLocated(By.css('.table-column-filter-popover')),
      5000,
    )
    const filterValue = await filterPopover.findElement(By.css('.filter-fields input[type="text"]'))
    await filterValue.sendKeys('appTitle')
    await filterPopover.findElement(By.xpath('.//button[normalize-space()="Add filter"]')).click()

    await driver.wait(async () => {
      const rows = await table.findElements(By.css('tbody tr'))
      if (rows.length !== 1) return false
      return (await rows[0].getText()).includes('appTitle')
    }, 10000)

    assert.ok(await table.findElement(By.css('tbody tr')).getText())
  })

  it('serves the OpenAPI document', async function () {
    await driver.get(base + '/openapi')
    await driver.wait(until.elementLocated(By.css('body')), 10000)
    const body = await driver.findElement(By.css('body')).getText()
    assert.match(body, /openapi|"paths"/i)
  })
})
