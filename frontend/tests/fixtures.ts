import {test as base,expect} from '@playwright/test'
export {expect}
export type {Page} from '@playwright/test'

// Keep real demo logins below the API's 10-per-minute limit. No auth mock or
// production rate-limit bypass: every browser still goes through /auth/login.
let nextTestAt=0
export const test=base.extend<{loginPacing:void}>({
 loginPacing:[async({},use)=>{
  const wait=Math.max(0,nextTestAt-Date.now())
  if(wait)await new Promise(resolve=>setTimeout(resolve,wait))
  nextTestAt=Date.now()+7500
  await use()
 },{auto:true}],
})
