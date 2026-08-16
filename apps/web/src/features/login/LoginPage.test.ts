import { describe, expect, it } from 'vitest';
import { resolveUsageServiceLoginMode, shouldAutoLoginUsageService } from './loginMode';

describe('resolveUsageServiceLoginMode', () => {
  it('keeps CPA-hosted panels on the regular login flow', () => {
    expect(resolveUsageServiceLoginMode(undefined)).toEqual({
      hostedByUsageService: false,
      usageServiceNeedsSetup: false,
    });
    expect(resolveUsageServiceLoginMode({ service: 'cli-proxy-api' })).toEqual({
      hostedByUsageService: false,
      usageServiceNeedsSetup: false,
    });
  });

  it('uses setup only for unconfigured Usage Service hosted panels', () => {
    expect(
      resolveUsageServiceLoginMode({ service: 'cpa-manager-plus', configured: false })
    ).toEqual({
      hostedByUsageService: true,
      usageServiceNeedsSetup: true,
    });
  });

  it('uses regular login for configured Usage Service hosted panels', () => {
    expect(resolveUsageServiceLoginMode({ service: 'cpa-manager-plus', configured: true })).toEqual(
      {
        hostedByUsageService: true,
        usageServiceNeedsSetup: false,
      }
    );
  });

  it('still recognizes legacy service ids (cpa-manager) as Usage Service', () => {
    expect(resolveUsageServiceLoginMode({ service: 'cpa-manager', configured: true })).toEqual({
      hostedByUsageService: true,
      usageServiceNeedsSetup: false,
    });
  });

  it('auto logs in only when a configured embedded service explicitly disables auth', () => {
    expect(
      shouldAutoLoginUsageService({
        service: 'cpa-manager-plus',
        configured: true,
        authDisabled: true,
      })
    ).toBe(true);
    expect(
      shouldAutoLoginUsageService({
        service: 'cpa-manager-plus',
        configured: false,
        authDisabled: true,
      })
    ).toBe(false);
    expect(shouldAutoLoginUsageService({ service: 'cpa-manager-plus', configured: true })).toBe(
      false
    );
  });
});
