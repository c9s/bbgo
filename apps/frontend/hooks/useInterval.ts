import { useEffect, useRef } from 'react';

export default function useInterval(cb: Function, delayMs: number | null) {
  const savedCallback = useRef<Function>();

  useEffect(() => {
    savedCallback.current = cb;
  }, [cb]);

  useEffect(() => {
    function tick() {
      savedCallback.current();
    }

    if (delayMs !== null) {
      let timerId = setInterval(tick, delayMs);
      return () => clearInterval(timerId);
    }
  }, [delayMs]);
}
