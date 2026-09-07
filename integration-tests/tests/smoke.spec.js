import assert from 'node:assert'
import { Builder, By, until } from 'selenium-webdriver'
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

  it('serves the OpenAPI document', async function () {
    await driver.get(base + '/openapi')
    await driver.wait(until.elementLocated(By.css('body')), 10000)
    const body = await driver.findElement(By.css('body')).getText()
    assert.match(body, /openapi|"paths"/i)
  })
})
