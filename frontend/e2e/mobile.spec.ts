import { expect, test, devices } from '@playwright/test'
import { login } from './helpers'

// Mobile drawer gestures: an edge swipe must open the drawer with live
// drag feedback (it used to stay unmounted until release, and the drag
// direction was inverted, so swipes visibly opened nothing). Short swipes
// and vertical scrolls must not open it, and tapping the backdrop closes it.
test.use({ ...devices['Pixel 7'] })

// Drag from the right edge (x) leftwards by `distance` px via CDP touch
// events, so React's touch handlers see real per-frame moves.
async function swipeFromEdge(page: import('@playwright/test').Page, distance: number, y = 400) {
  const client = await page.context().newCDPSession(page)
  const x = page.viewportSize()!.width - 5 // inside the right 24px trigger zone
  await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y }] })
  for (let i = 1; i <= 5; i++) {
    await client.send('Input.dispatchTouchEvent', {
      type: 'touchMove',
      touchPoints: [{ x: x - (distance * i) / 5, y }],
    })
  }
  await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
}

// The drawer renders its own nav; the desktop sidebar nav is hidden below md
// but stays in the DOM, so the last Dashboard link is the drawer's one.
const drawerNav = (page: import('@playwright/test').Page) =>
  page.getByRole('link', { name: 'Dashboard' }).last()

test('edge swipe opens the drawer with the drawer following the finger', async ({ page }) => {
  await login(page)
  const client = await page.context().newCDPSession(page)
  const x = page.viewportSize()!.width - 5
  await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y: 400 }] })
  await client.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: x - 85, y: 400 }] })
  // Mid-gesture the drawer must already be mounted and partially on screen.
  await expect(drawerNav(page)).toBeVisible()
  await expect(drawerNav(page)).toBeInViewport()
  await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
  // Released past the halfway point: stays open.
  await expect(drawerNav(page)).toBeVisible()
  await expect(drawerNav(page)).toBeInViewport()
})

test('a short edge swipe releases back to closed', async ({ page }) => {
  await login(page)
  await swipeFromEdge(page, 80) // well under the halfway mark (128px)
  // The close animation unmounts the drawer ~320ms later.
  await expect(drawerNav(page)).toBeHidden()
})

test('vertical scrolling on the right edge does not open the drawer', async ({ page }) => {
  await login(page)
  const client = await page.context().newCDPSession(page)
  const x = page.viewportSize()!.width - 5
  await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y: 300 }] })
  await client.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x, y: 500 }] })
  await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
  await expect(drawerNav(page)).toBeHidden()
})

test('hamburger opens the drawer and tapping the backdrop closes it', async ({ page }) => {
  await login(page)
  await page.getByRole('button', { name: /menu/i }).click()
  await expect(drawerNav(page)).toBeVisible()
  // Tap the backdrop (left of the drawer).
  await page.mouse.click(40, 400)
  await expect(drawerNav(page)).toBeHidden()
})
