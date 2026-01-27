import { Slider as AntSlider } from 'antd';

export const Slider = ({
  isEnabled,
  value,
  onChange,
  onChangeStart,
  onChangeEnd,
}: {
  isEnabled: boolean;
  direction?: 'horizontal' | 'vertical';
  value: number;
  onChangeStart?: () => void;
  onChange: (value: number) => void;
  onChangeEnd?: (value: number) => void;
}) => {
  const handleChange = (newValue: number | number[]) => {
    const val = Array.isArray(newValue) ? newValue[0] : newValue;
    onChange(val / 100); // Convert from 0-100 to 0-1
  };

  const handleAfterChange = (newValue: number | number[]) => {
    const val = Array.isArray(newValue) ? newValue[0] : newValue;
    onChangeEnd?.(val / 100);
  };

  return (
    <div className='volume-sider-container'>
      <AntSlider
        disabled={!isEnabled}
        value={value * 100} // Convert from 0-1 to 0-100
        onChange={handleChange}
        // onChangeStart={onChangeStart}
        onChangeComplete={handleAfterChange}
        tooltip={{ formatter: null }} // Hide tooltip
        className='volume-sider'
        style={{ cursor: 'pointer' }}
      />
    </div>
  );
};
