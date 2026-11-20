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
        usageMode: 'Usage rebates use the rate of your current tier as well; tiers still follow recharges.'
      }
    },
    affiliateMode: {
      title: 'Rebate basis',
      description:
        'Recharge: invitees earn their inviter a rebate whenever they recharge (the original behaviour). Usage: recharges earn nothing; instead, once an hour, the balance usage of each invitee in the previous hour is rebated, and usage within subscriptions does not count. Both use the exclusive rate, then the tiers, then the global rate, with tiers still based on recharges; the per-invitee cap, rebate duration and freeze period apply.',
      recharge: 'Recharge',
      usage: 'Usage',
      since: 'Usage rebates started at {time}; balance usage from then on is settled.',
      until: 'Usage rebates stopped at {time}; usage before then is still settled.',
      settleHint: 'Each hour is settled about 5 minutes after it ends, with hours on the clock of the server time zone.',
      settledUntil: 'Settled up to {time}.',
      confirmTitle: 'Switch rebate basis',
      confirmToUsage: 'After switching to usage, recharges earn no rebate and the balance usage of invitees from now on is settled every hour. Switch?',
      confirmToRecharge: 'After switching back to recharge, usage from now on earns no rebate (earlier usage is still settled) and recharges earn rebates again. Switch?',
      confirm: 'Switch',
      cancel: 'Cancel',
      switched: 'Rebate basis switched',
      switchFailed: 'Could not switch the rebate basis',
      loadFailed: 'Could not load the rebate basis',
      retry: 'Retry',
      recentDays: 'Recent settlements (per day)',
      viewRecords: 'View records',
      skippedPeriods: '{count} hours without rebates: affiliate rebates were off',
      noDays: 'Nothing settled yet',
      columns: {
        day: 'Day',
        invitees: 'Invitees',
        usage: 'Usage',
        rebate: 'Rebate'
      }
    },
    affiliateUsage: {
      title: 'Usage rebate records',
      adminHint: 'One record per invitee and settlement period, newest first. A period is usually an hour; the hour of a switch of the rebate basis is shorter.',
      userHint: 'The hourly balance usage of the users you invited and the rebate it earned you, newest first.',
      searchPlaceholder: 'Search inviter or invitee by email or username',
      search: 'Search',
      loadFailed: 'Could not load the records',
      retry: 'Retry',
      empty: 'Nothing settled yet',
      noMatch: 'No matching records',
      columns: {
        period: 'Period',
        inviter: 'Inviter',
        invitee: 'Invitee',
        usage: 'Usage',
        rate: 'Rate',
        rebate: 'Rebate'
      },
      user: {
        title: 'Usage rebates',
        usageOn: 'When the users you invited spend their balance, you earn a rebate at your rebate rate, settled every hour.',
        usageOff: 'Rebates now follow recharges; the records of earlier usage rebates are kept here.',
        latest: 'Latest: {period}, rebate {amount}',
        none: 'Nothing settled yet.',
        view: 'View records ({count})',
        history: 'The records of your earlier usage rebates are kept here.'
      }
    },
    affiliateScope: {
      title: 'Who it is on for',
      description:
        'Affiliate rebates can be on for every user or only for selected users. With selected users only, everyone else has no affiliate entry in the menu, their invite code is not valid, and the recharges and usage of the users they invited earn them nothing; rebate quota they already have can still be transferred to the balance.',
      all: 'All users',
      selected: 'Selected users only',
      summarySelected: 'On for {count} users only.',
      summaryAll: 'On for all users. {count} users are selected; with "Selected users only" it is on for them alone.',
      howTo: 'Edit a user under Users to switch affiliate rebates on or off for that user.',
      listTitle: 'Selected users',
      listFailed: 'Could not load the users',
      columns: {
        user: 'User',
        selectedAt: 'Switched on'
      },
      remove: 'Switch off',
      removed: 'Affiliate rebates switched off for {user}',
      removeFailed: 'Could not switch it off',
      confirmTitle: 'Change who it is on for',
      confirmToSelected: 'After the change only the {count} selected users have affiliate rebates; the invite codes of all other users stop working, and the recharges and usage of the users they invited earn them nothing. Change it?',
      confirmToAll: 'After the change affiliate rebates are on for all users. Change it?',
      confirm: 'Change',
      cancel: 'Cancel',
      switched: 'Changed who affiliate rebates are on for',
      switchFailed: 'Could not change who affiliate rebates are on for',
      loadFailed: 'Could not load who affiliate rebates are on for',
      retry: 'Retry',
      user: {
        label: 'Affiliate rebates',
        hintOn: 'On for this user: the affiliate page shows, the invite code is valid and rebates are earned.',
        hintOff: 'Off for this user: the invite code is not valid, and the recharges and usage of the users they invited earn them nothing.',
        hintScopeAll: 'Affiliate rebates are on for all users, so this switch has no effect yet. Once the system settings limit them to selected users, only users with this switch on have affiliate rebates.',
        hintFeatureOff: 'Affiliate rebates are switched off altogether, so this switch has no effect for now.',
        instant: 'The switch takes effect at once, without Update.',
        loadFailed: 'Could not load the affiliate switch',
        saveFailed: 'Could not save the affiliate switch',
        switchedOn: 'Affiliate rebates switched on for this user',
        switchedOff: 'Affiliate rebates switched off for this user'
      },
      notice: {
        title: 'Affiliate rebates are not on for your account',
        body: 'Your invite code is not valid at the moment, and the recharges and usage of the users you invited earn you no new rebates. Rebate quota you already have can still be transferred to your balance.'
      }
    },
    smsOps: {
      title: 'SMS alerts',
      enabled: 'Enable SMS alerts',
      phones: 'Alert phones',
      phonePlaceholder: 'Phone number',
      add: 'Add',
      remove: 'Remove',
      hint: 'Mainland China mobile numbers only, at most 10. Without a phone, SMS alerts switch off. Alert messages count towards the daily cap of the SMS notifications, per phone.',
      minLevel: 'Lowest level',
      notReady: 'No SMS can go out yet: enable SMS and set the ops alert template ID in System settings → Email & SMS → SMS notifications first.',
      loadFailed: 'Failed to load the SMS alert settings',
      retry: 'Retry',
      invalidPhone: 'Enter a mainland China mobile number',
      duplicate: 'This number is already in the list',
      tooMany: 'At most 10 phone numbers',
      saveFailed: 'Failed to save the SMS alert settings'
    },
    smsQuota: {
      phones: 'Notification phones',
      hint: 'When an account reaches its alert line, these numbers get an SMS as well. Mainland China mobile numbers only, at most 10; leave empty for no SMS.',
      phonePlaceholder: 'Phone number',
      add: 'Add phone',
      remove: 'Remove',
      notReady: 'No SMS can go out yet: enable SMS and set the account quota template ID in the SMS notifications card below first.',
      loadFailed: 'Failed to load the notification phones',
      retry: 'Retry',
      invalidPhone: 'Not a mainland China mobile number: {phone}',
      tooMany: 'At most 10 notification phones',
      saveFailed: 'Failed to save the notification phones'
    },
    sms: {
      title: 'SMS notifications',
      description:
        'Send low balance reminders, subscription expiry reminders, and ops alerts and account quota alerts for admins through Tencent Cloud SMS. SMS and email are independent: users with a bound phone get SMS, users with notification emails get email, users with both get both. Admin alert phones are set next to the alert emails: ops alerts in Ops monitoring → Settings, account quota alerts in the Account quota notification card above. Recharge, subscription purchase and content moderation notices are never sent by SMS.',
      enabled: 'Enable SMS notifications',
      dryRun: 'Log only, do not send',
      dryRunHint:
        'Messages are only written to the send log, not sent, so the whole flow can be tried without Tencent Cloud settings. Verification codes show in the send log.',
      provider: 'Tencent Cloud SMS',
      providerHint:
        'Create an application and apply for a signature and templates in the Tencent Cloud SMS console, then fill them in here once approved. The SecretKey is not shown again after saving.',
      secretId: 'SecretId',
      secretKey: 'SecretKey',
      secretKeySet: 'Set; leave empty to keep it',
      sdkAppId: 'SDK AppID',
      signName: 'Signature',
      signNamePlaceholder: 'The approved signature text',
      region: 'Region',
      regions: {
        guangzhou: 'South China (Guangzhou)',
        beijing: 'North China (Beijing)',
        nanjing: 'East China (Nanjing)'
      },
      templates: 'SMS templates',
      templatesHint:
        'Templates must be approved by Tencent Cloud first; the suggested templates below can serve as a starting point. The number and order of variables must match the suggested template. Each variable holds at most 20 characters; longer values are cut.',
      columns: {
        send: 'Send',
        templateId: 'Template ID'
      },
      templateIdPlaceholder: 'Template ID',
      suggested: 'Suggested template',
      params: 'Variables in order: {params}',
      copy: 'Copy template',
      copied: 'Template copied',
      kinds: {
        verify_code: {
          name: 'Verification code',
          hint: 'Used when users bind a phone number on their profile page; without it, no phone can be bound.',
          switchedAt: 'On binding',
          params: 'code, minutes valid'
        },
        balance_low: {
          name: 'Low balance reminder',
          hint: 'Sent when a balance drops below the reminder threshold; the switch and threshold of the low balance reminder above apply. At most one a day per user.',
          params: 'current balance, threshold'
        },
        subscription_expiry: {
          name: 'Subscription expiry reminder',
          hint: 'Sent to the bound phone 7, 3 and 1 days before a subscription expires, independently of the reminder email.',
          params: 'subscription name, expiry time, days left'
        },
        ops_alert: {
          name: 'Ops alert',
          hint: 'Sent when an ops monitoring alert rule fires; alert silencing applies. The switch, phones and lowest level are in Ops monitoring → Settings, below the alert emails.',
          switchedAt: 'Switched on in the ops monitoring settings',
          params: 'rule name, severity, current value, threshold'
        },
        account_quota: {
          name: 'Account quota alert',
          hint: 'Sent when an account reaches its quota alert line. The phones are below the notification emails of the Account quota notification card above and follow its switch; each account sets its alert thresholds when edited. At most one a day per account and quota.',
          switchedAt: 'Phones set in the account quota notification',
          params: 'account name, quota kind, remaining, limit'
        }
      },
      dailyLimit: 'Daily cap',
      dailyLimitHint:
        'Notification SMS a user gets a day at most, counted per phone for admin alerts; 0 means no cap. Verification codes and test messages do not count.',
      dailyLimitUnit: 'messages',
      save: 'Save SMS settings',
      saving: 'Saving…',
      saved: 'SMS settings saved',
      saveFailed: 'Failed to save the SMS settings',
      unsaved: 'Unsaved changes',
      loadFailed: 'Failed to load the SMS settings',
      retry: 'Retry',
      showLogs: 'Send log',
      test: {
        title: 'Send a test message',
        hint: 'Fills the chosen template with sample values and sends it to this number using the saved settings; works before SMS notifications are enabled.',
        phonePlaceholder: 'Phone number to receive the test',
        send: 'Send test',
        sending: 'Sending…',
        saveFirst: 'Save the settings before testing',
        sent: 'Test message sent',
        dryRun: 'Written to the send log (log only mode)',
        failed: 'Sending failed: {error}',
        requestFailed: 'Failed to send the test message'
      },
      validation: {
        templateId: 'A template ID is a number such as 1234567',
        appId: 'The SDK AppID is a number such as 1400000000',
        dailyLimit: 'The daily cap must be between 0 and 1000',
        credentials: 'Fill in SecretId, SecretKey, SDK AppID and the signature before enabling, or turn on log only'
      },
      log: {
        title: 'SMS send log',
        hint: 'Messages of the last 90 days, newest first. Verification codes are not stored, except in log only mode.',
        searchPlaceholder: 'Search by phone or user email',
        search: 'Search',
        empty: 'Nothing sent yet',
        noMatch: 'No matching messages',
        loadFailed: 'Failed to load the send log',
        retry: 'Retry',
        admin: 'Admin',
        columns: {
          time: 'Time',
          event: 'Kind',
          phone: 'Phone',
          user: 'Recipient',
          content: 'Template and variables',
          status: 'Status'
        },
        events: {
          verify_code: 'Verification code',
          test: 'Test',
          balance_low: 'Low balance',
          subscription_expiry: 'Subscription expiry',
          ops_alert: 'Ops alert',
          account_quota: 'Account quota'
        },
        statuses: {
          pending: 'Sending',
          sent: 'Sent',
          failed: 'Failed',
          dry_run: 'Logged only',
          limited: 'Over the daily cap'
        }
      },
      user: {
        title: 'SMS reminders',
        description:
          'Once a phone is bound, {events} are also sent to it by SMS, independently of the notification emails. One mainland China mobile number can be bound.',
        and: ' and ',
        events: {
          balance_low: 'low balance reminders',
          subscription_expiry: 'subscription expiry reminders'
        },
        phonePlaceholder: 'Phone number',
        sendCode: 'Get code',
        resend: 'Resend',
        countdown: 'Resend in {seconds}s',
        codePlaceholder: '6-digit code',
        bind: 'Verify and bind',
        cancel: 'Cancel',
        verified: 'Verified',
        remove: 'Unbind',
        codeSent: 'Code sent to {phone}',
        bound: 'Phone bound',
        removed: 'Phone unbound',
        paused: 'SMS reminders paused',
        resumed: 'SMS reminders resumed',
        sendFailed: 'Failed to send the code',
        bindFailed: 'Binding failed',
        updateFailed: 'Update failed',
        loadFailed: 'Failed to load the SMS reminders'
      },
      errors: {
        MOK_SMS_UNAVAILABLE: 'SMS reminders are not available',
        MOK_SMS_PHONE_INVALID: 'Enter a mainland China mobile number',
        MOK_SMS_CODE_COOLDOWN: 'Too frequent; try again in {seconds} seconds',
        MOK_SMS_CODE_DAILY_LIMIT: 'No more codes today; try again tomorrow',
        MOK_SMS_CODE_INVALID: 'The code is wrong',
        MOK_SMS_CODE_EXPIRED: 'The code has expired; get a new one',
        MOK_SMS_SEND_FAILED: 'The SMS could not be sent; try again later',
        MOK_SMS_PHONE_NOT_BOUND: 'No phone number is bound',
        MOK_SMS_NOT_CONFIGURED: 'Fill in and save the Tencent Cloud settings and the template ID first'
      }
    },
    paymentQuickAmounts: {
      title: 'Recharge quick amounts',
      description:
        'The amount buttons users can pick on the recharge page. Without a list, the default {defaults} are shown. Buttons outside the payment methods\' minimum and maximum are still hidden, and recharge bonus tiers still show their tags on the matching buttons.',
      label: 'Quick amounts',
      placeholder: 'For example: 30, 50, 100, 200, 500',
      hint: 'Separate with commas or spaces; at most 12 amounts with up to 2 decimals, sorted ascending when saved. Save an empty field to restore the defaults.',
      preview: 'Preview',
      usingDefault: '(defaults)',
      save: 'Save',
      saving: 'Saving…',
      saved: 'Quick amounts saved',
      saveFailed: 'Failed to save the quick amounts',
      reset: 'Restore defaults',
      resetDone: 'Default quick amounts restored',
      unsaved: 'Unsaved changes',
      loadFailed: 'Failed to load the quick amounts',
      retry: 'Retry',
      errors: {
        invalid: '"{value}" is not a number',
        range: '"{value}" is out of range: amounts must be greater than 0 and at most 1000000',
        decimals: '"{value}" has more than 2 decimals',
        tooMany: 'At most {max} amounts'
      }
    }
  }
}
