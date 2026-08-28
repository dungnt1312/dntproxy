import { useEffect, useRef } from 'react';
import { useBlocker } from 'react-router-dom';
import { confirmLeaveSetup } from './helpers';

export function useAddConnectionGuard(shouldGuard: boolean) {
    const intentionalNavRef = useRef(false);

    useEffect(() => {
        if (!shouldGuard) return;
        const handler = (event: BeforeUnloadEvent) => {
            event.preventDefault();
            event.returnValue = '';
        };
        window.addEventListener('beforeunload', handler);
        return () => window.removeEventListener('beforeunload', handler);
    }, [shouldGuard]);

    const blocker = useBlocker(
        ({ currentLocation, nextLocation }) =>
            !intentionalNavRef.current && shouldGuard && currentLocation.pathname !== nextLocation.pathname,
    );

    useEffect(() => {
        if (blocker.state !== 'blocked') return;
        if (confirmLeaveSetup()) {
            blocker.proceed();
            // Reset so future accidental navigations stay guarded; without this the
            // flag stayed true forever after the first intentional nav.
            intentionalNavRef.current = false;
        } else {
            blocker.reset();
        }
    }, [blocker]);

    return {
        markIntentionalNav: () => {
            intentionalNavRef.current = true;
        },
    };
}
