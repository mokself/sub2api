export default {
  mok: {
    claudePlan: {
      column: 'Claude Plan',
      refresh: 'Fetch the plan again',
      failed: 'Failed',
      requestFailed: 'Could not load the plan; use the button on the right to retry',
      updatedAt: 'Updated {time}',
      attemptedAt: 'Last attempt {time}',
      lastError: 'Last fetch failed: {message}',
      unsupportedScope: 'The authorization lacks the user:profile scope; re-authorize the account to read its plan',
      unsupportedOther: 'The plan of this account cannot be read',
      subscriptionStatus: 'Subscription {status}'
    },
    affiliateTiers: {
      title: 'Tiered rebate',
      description:
        "An inviter's rebate rate grows with their team volume: once the team volume, including the current recharge, reaches a tier, that whole recharge is rebated at the tier's rate. Below the first tier the global rebate rate above applies; users with an exclusive rate keep it. Without tiers everyone gets the global rate.",
      volumeHint:
        'Team volume = the recharges of the inviter and everyone they invited: completed, unrefunded online payment orders (credited amount without recharge bonus) plus used balance redeem codes. Balance added by admins does not count. Amounts use the balance unit.',
      minAmountLabel: 'Team volume reaches',
      rateLabel: 'Rebate rate',
      addTier: 'Add tier',
      removeTier: 'Remove this tier',
      empty: 'No tiers yet; every inviter gets the global rebate rate.',
      maxTiers: 'At most {max} tiers',
      save: 'Save tiers',
      saving: 'Saving…',
      saved: 'Tiered rebate saved',
      unsaved: 'Unsaved changes',
      loadFailed: 'Could not load the tiered rebate settings',
      saveFailed: 'Could not save the tiered rebate settings',
      retry: 'Retry',
      errors: {
        minAmountInvalid: 'The threshold must be greater than 0',
        rateInvalid: 'The rebate rate must be between 0 and 100',
        minAmountDuplicate: 'Another tier has the same threshold'
      },
      user: {
        title: 'Tiered rebate',
        currentRate: 'Current rebate rate',
        teamVolume: 'Team volume',
        teamVolumeHint: 'Credited recharges of you and the users you invited (without recharge bonus, redeem codes included)',
        nextTier: '{amount} more to reach a {rate}% rebate',
        topTier: 'Top tier reached',
        ladder: 'Rebate tiers',
        baseRow: 'Below {amount}',
        tierRow: 'From {amount}',
        current: 'Current',
        usageMode: 'Rebates follow the daily balance usage of your invitees and are settled the next day; tiers still follow recharges.'
      }
    },
    affiliateMode: {
      title: 'Rebate basis',
      description:
        'Recharge: invitees earn their inviter a rebate whenever they recharge (the original behaviour). Usage: recharges earn nothing; instead, once a day, the balance usage of each invitee on the previous day is rebated, and usage within subscriptions does not count. Both use the exclusive rate, then the tiers, then the global rate, with tiers still based on recharges; the per-invitee cap, rebate duration and freeze period apply.',
      recharge: 'Recharge',
      usage: 'Usage',
      since: 'Usage rebates started at {time}; balance usage from then on is settled.',
      until: 'Usage rebates stopped at {time}; usage before then is still settled.',
      settleHint: 'Each day is settled after 0:30 the next day, with days in the server time zone.',
      confirmTitle: 'Switch rebate basis',
      confirmToUsage: 'After switching to usage, recharges earn no rebate and the balance usage of invitees from now on is settled daily. Switch?',
      confirmToRecharge: 'After switching back to recharge, usage from now on earns no rebate (earlier usage is still settled) and recharges earn rebates again. Switch?',
      confirm: 'Switch',
      cancel: 'Cancel',
      switched: 'Rebate basis switched',
      switchFailed: 'Could not switch the rebate basis',
      loadFailed: 'Could not load the rebate basis',
      retry: 'Retry',
      recentDays: 'Recent settlements',
      noDays: 'Nothing settled yet',
      columns: {
        day: 'Day',
        invitees: 'Invitees',
        usage: 'Usage',
        rebate: 'Rebate'
      },
      skipped: {
        affiliate_disabled: 'Affiliate rebates were off; nothing paid'
      }
    }
  }
}
